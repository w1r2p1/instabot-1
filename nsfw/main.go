package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/go-redis/redis"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"gitlab.com/nuxdie/instabot/metadata"
	"github.com/hashicorp/logutils"
	"os"
)

const envLogLevel = "LOG_LEVEL"
const envWorkerRedisAddr = "WORKER_REDIS_ADDR"
const envWorkerRedisDb = "WORKER_REDIS_DB"
const envWorkerRedisPasswd = "WORKER_REDIS_PASSWD"
const envWorkerRedisChannel = "WORKER_REDIS_CHANNEL"
const envWorkerNSFWApiUrl = "WORKER_NSFW_API_URL"
const envWorkerNSFWApiKey = "WORKER_NSFW_API_KEY"

type Worker struct {
	redis *redis.Client
	config *workerConfig
}

type workerConfig struct {
	nsfwApi struct{
		url string
		key string
	}
	redis struct{
		channel string
		addr string
		passwd string
		db int
	}
}

type NSFWApiResponse struct {
	Output struct{
		Nsfw_score float32
	}
	Job_id int
	Err string
}

func main() {
	worker := NewWorker()
	worker.Start()
}

func NewWorker() *Worker {
	var worker Worker

	worker.config = config()

	if len(worker.config.nsfwApi.key) == 0 {
		log.Fatal("[FATAL] Couldn't create worker due to laking NSFW api key")
	}

	worker.redis = redis.NewClient(&redis.Options{
		Addr: worker.config.redis.addr,
		Password: worker.config.redis.passwd,
		DB: worker.config.redis.db,
	})

	worker.setupRedis()

	return &worker
}

func (worker Worker) Start() {

}

func config() *workerConfig {
	viper.AutomaticEnv()
	viper.SetDefault(envWorkerNSFWApiUrl, "https://api.deepai.org/api/nsfw-detector")
	viper.SetDefault(envWorkerRedisAddr, "localhost:6379")
	viper.SetDefault(envWorkerRedisPasswd, "")
	viper.SetDefault(envWorkerRedisChannel, "queue")
	viper.SetDefault(envWorkerRedisDb, 0)
	viper.SetDefault(envLogLevel, "WARN")

	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"},
		MinLevel: logutils.LogLevel(viper.GetString(envLogLevel)),
		Writer: os.Stderr,
	}
	log.SetOutput(filter)

	conf := &workerConfig{}

	conf.nsfwApi.url = viper.GetString(envWorkerNSFWApiUrl)
	conf.nsfwApi.key = viper.GetString(envWorkerNSFWApiKey)

	conf.redis.addr = viper.GetString(envWorkerRedisAddr)
	conf.redis.passwd = viper.GetString(envWorkerRedisPasswd)
	conf.redis.channel = viper.GetString(envWorkerRedisChannel)
	conf.redis.db = viper.GetInt(envWorkerRedisDb)

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

	defer pubsub.Close()
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

	if !updateMsg.NSFWChecked {
		res, err := worker.redis.HGetAll(updateMsg.PhotoId).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't hget from redis for ID %s: %s",
				updateMsg.PhotoId, err)
		}
		log.Printf("[DEBUG] Got from redis: %v", res)

		var metaFromRedis metadata.PhotoMetadata
		err = mapstructure.WeakDecode(res, &metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't map response from API to metadata struct: %s", err)
			return
		}

		log.Printf("[DEBUG] got metadata from redis: %v", metaFromRedis)

		if metaFromRedis.NSFWChecked {
			log.Printf("[INFO] Nothing to do. Already has checked for NSFW: %s",
				metaFromRedis.NSFW)

			meta, err := json.Marshal(&metaFromRedis)

			if err != nil {
				log.Printf("[ERROR] Couldn't encode JSON: %s", err)
			}
			_, err = worker.redis.Publish(worker.config.redis.channel, meta).Result()

			if err != nil {
				log.Printf("[ERROR] Couldn't publish photo metadata to redis channel: %s",
					worker.config.redis.channel, err)
			}
			return
		}
		nsfw, err := worker.process(metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't get nsfw from API: %s", err)
			return
		}

		metaFromRedis.NSFWChecked = true

		_, err = worker.redis.HSet(updateMsg.PhotoId, "nsfw_checked", true).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set nsfw_checked in redis for %s: %s",
				updateMsg.PhotoId, err)
		}

		metaFromRedis.NSFW = nsfw

		_, err = worker.redis.HSet(updateMsg.PhotoId, "nsfw", nsfw).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set nsfw in redis for %s: %s",
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

func (worker Worker) process(photoMetadata metadata.PhotoMetadata) (bool, error) {
	var postData bytes.Buffer

	resp := getPhoto(photoMetadata.PhotoUrl)

	w := multipart.NewWriter(&postData)

	fw, err := w.CreateFormFile("image", "file.jpg")

	if err != nil {
		log.Printf("[ERROR] Couldn't create image form field: %s", err)
	}

	_, err = io.Copy(fw, resp.Body)

	if err != nil {
		log.Printf("[ERROR] Couldn't write image to field: %s", err)
	}

	resp.Body.Close()
	w.Close()

	req, err := http.NewRequest("POST", worker.config.nsfwApi.url, &postData)

	if err != nil {
		log.Printf("[ERROR] Couldn't create http request: %s", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Api-Key", worker.config.nsfwApi.key)

	client := &http.Client{}

	res, err := client.Do(req)

	if err != nil {
		log.Printf("[ERROR] Couldn't make a request to nsfw api: %s", err)
	}

	var nsfwApiResponse NSFWApiResponse

	err = json.NewDecoder(res.Body).Decode(&nsfwApiResponse)

	if err != nil {
		log.Printf("[ERROR] Couldn't read response from api: %s", err)
	}

	defer res.Body.Close()

	var captionErr error
	var decision bool

	if len(nsfwApiResponse.Err) != 0 {
		captionErr = errors.New(nsfwApiResponse.Err)
	} else {
		captionErr = nil
	}

	decision = nsfwApiResponse.Output.Nsfw_score > 0.5

	return decision, captionErr
}

func getPhoto(uri string) * http.Response {
	parsed, err := url.Parse(uri)

	if parsed.Host == "" || err != nil {
		log.Printf("[ERROR] Incorrect photo url provided: %s", uri)
	}

	resp, err := http.Get(uri)

	if err != nil {
		log.Printf("[ERROR] Could not get the photo by %s", uri)
	}

	return resp
}
