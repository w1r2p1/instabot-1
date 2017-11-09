package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/ahmdrz/goinsta"
	"github.com/ahmdrz/goinsta/response"
	"github.com/go-redis/redis"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"gitlab.com/nuxdie/instabot/metadata"
)

const envWorkerRedisAddr = "WORKER_REDIS_ADDR"
const envWorkerRedisDb = "WORKER_REDIS_DB"
const envWorkerRedisPasswd = "WORKER_REDIS_PASSWD"
const envWorkerRedisChannel = "WORKER_REDIS_CHANNEL"
const envWorkerInstagramUsername = "WORKER_INSTAGRAM_USERNAME"
const envWorkerInstagramPassword = "WORKER_INSTAGRAM_PASSWORD"

type Worker struct {
	redis *redis.Client
	insta goinsta.Instagram
	config *workerConfig
}

type workerConfig struct {
	instagram struct{
		username string
		password string
	}
	redis struct{
		channel string
		addr string
		passwd string
		db int
	}
}

func main() {
	worker := NewWorker()
	worker.Start()
}

func NewWorker() *Worker {
	var worker Worker

	worker.config = config()

	worker.redis = redis.NewClient(&redis.Options{
		Addr: worker.config.redis.addr,
		Password: worker.config.redis.passwd,
		DB: worker.config.redis.db,
	})

	insta, err := worker.loginInstagram()

	if err != nil {
		log.Fatalf("[ERROR] Couldn't login to instagram: %s", err)
	}

	worker.insta = *insta

	worker.setupRedis()

	return &worker
}

func (worker Worker) Start() {

}

func config() *workerConfig {
	viper.AutomaticEnv()
	viper.SetDefault(envWorkerRedisAddr, "localhost:6379")
	viper.SetDefault(envWorkerRedisPasswd, "")
	viper.SetDefault(envWorkerRedisChannel, "queue")
	viper.SetDefault(envWorkerRedisDb, 0)

	conf := &workerConfig{}

	conf.redis.addr = viper.GetString(envWorkerRedisAddr)
	conf.redis.passwd = viper.GetString(envWorkerRedisPasswd)
	conf.redis.channel = viper.GetString(envWorkerRedisChannel)
	conf.redis.db = viper.GetInt(envWorkerRedisDb)

	conf.instagram.username = viper.GetString(envWorkerInstagramUsername)
	conf.instagram.password = viper.GetString(envWorkerInstagramPassword)

	return conf
}

func (worker Worker) setupRedis() {
	pong, err := worker.redis.Ping().Result()

	if err != nil {
		log.Printf("[ERROR] Couldn't ping redis server %s", err)
	} else {
		log.Printf("[DEBUG] got pong from redis %v", pong)
	}

	pubsub := worker.redis.Subscribe(worker.config.redis.channel)
	ch := pubsub.Channel()

	subscr, err := pubsub.ReceiveTimeout(time.Second*time.Duration(10))

	defer pubsub.Close()

	if err != nil {
		log.Fatalf("[ERROR] Couldn't subscribe to redis channel %s: %s",
			worker.config.redis.channel, err)
	}

	log.Printf("[INFO] subscribed to redis channel %s: %v",
		worker.config.redis.channel, subscr)

	for message := range ch {
		go func(messageVal *redis.Message) {
			worker.handleRedis(messageVal)
		}(message)
	}
}

func (worker Worker) handleRedis(message *redis.Message) {
	log.Printf("[DEBUG] Got message from redis channel %s: %v",
		worker.config.redis.channel, message)

	var updateMsg metadata.PhotoMetadata
	err := json.Unmarshal([]byte(message.Payload), &updateMsg)

	if err != nil {
		log.Printf("[ERROR] Couldn't decode JSON metadata, %s", message.Payload)
	}

	log.Printf("[DEBUG] Got metadata from message %v", updateMsg)

	if updateMsg.Publish {
		metaHGet, err := worker.redis.HGetAll(updateMsg.PhotoId).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't hget from redis for ID %s: %s",
				updateMsg.PhotoId, err)
		}
		log.Printf("[DEBUG] Got from redis: %v", metaHGet)

		var metaFromRedis metadata.PhotoMetadata
		err = mapstructure.WeakDecode(metaHGet, &metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't map response from API to metadata struct: %s",
				err)
			return
		}

		log.Printf("[DEBUG] got metadata from redis: %v", metaFromRedis)

		if metaFromRedis.Published {
			log.Printf("[INFO] Nothing to do. Already has published status: %s, %b",
				metaFromRedis.Publish, metaFromRedis.Published)
			return
		}

		mediaCodeRes, err := worker.process(metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't get status from API: %s", err)
			return
		}

		status := len(metaHGet) != 0
		metaFromRedis.Published = status
		metaFromRedis.PublishedUrl = "https://www.instagram.com/p/" + mediaCodeRes

		_, err = worker.redis.HSet(updateMsg.PhotoId, "published", status).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set status in redis for %s: %s",
				updateMsg.PhotoId, err)
		}
		_, err = worker.redis.HSet(updateMsg.PhotoId, "published_url",
			metaFromRedis.PublishedUrl).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set status in redis for %s: %s",
				updateMsg.PhotoId, err)
		}

		meta, err := json.Marshal(&metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't encode JSON: %s", err)
		}
		_, err = worker.redis.Publish(worker.config.redis.channel, meta).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't publish photo metadata to redis channel: %s",
				worker.config.redis.channel, err)
		}
	}
}

func (worker Worker) process(photoMetadata metadata.PhotoMetadata) (string, error) {
	resp, err := getPhoto(photoMetadata.PhotoUrl)

	if err != nil {
		log.Printf("[ERROR] Couldn't get photo: %s", err)
		return "", err
	}

	res, err := worker.upload(resp.Body, photoMetadata.FinalCaption)

	if err != nil {
		log.Printf("[ERROR] Couldn't upload photo %s to Instagram: %s",
			photoMetadata.PhotoId, err)
		return "", err
	}

	return res.Media.Code, nil
}

func (worker Worker) loginInstagram() (*goinsta.Instagram, error) {
	if len(worker.config.instagram.username)*len(worker.config.instagram.password) == 0 {
		log.Fatalf("[ERROR] Please provide valid instagram username and password")
	}

	insta := goinsta.New(worker.config.instagram.username, worker.config.instagram.password)

	if err := insta.Login(); err != nil {
		log.Fatalf("[ERROR] Couldn't login to Instagram %s", err)
		return nil, err
	}

	log.Printf("[INFO] Logged in to Instagram")

	return insta, nil
}

func getPhoto(uri string) (*http.Response, error) {
	parsed, err := url.Parse(uri)

	if parsed.Host == "" || err != nil {
		log.Printf("[ERROR] Incorrect photo url %s provided: %s", uri, err)
		return nil, err
	}

	resp, err := http.Get(uri)

	if err != nil {
		log.Printf("[ERROR] Could not get the photo by %s", uri)
		return nil, err
	}

	return resp, nil
}


func (worker Worker) upload(photo io.ReadCloser, caption string) (
	response.UploadPhotoResponse, error) {

	insta, err := worker.loginInstagram()

	quality := 87
	uploadId := worker.insta.NewUploadID()
	filterType := goinsta.Filter_Valencia

	uploadPhotoResponse, err := insta.UploadPhotoFromReader(photo,
		caption, uploadId, quality, filterType)

	if err != nil {
		log.Printf("[ERROR] Couldn't upload photo to instagram: %s", err)
		return uploadPhotoResponse, err
	}

	defer insta.Logout()

	return uploadPhotoResponse, nil
}

func disableComments(insta *goinsta.Instagram,
	uploadPhotoResponse response.UploadPhotoResponse) {
	// TODO use this functionality if config says so
	_, err := insta.DisableComments(uploadPhotoResponse.Media.ID)

	if err != nil {
		panic(fmt.Sprintf("Error trying to disable comments for mediaId %s: %s",
			uploadPhotoResponse.Media.ID, err))
	}
}