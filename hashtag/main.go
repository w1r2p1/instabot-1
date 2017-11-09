package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/vision/apiv1"
	"github.com/go-redis/redis"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

const envWorkerRedisAddr = "WORKER_REDIS_ADDR"
const envWorkerRedisDb = "WORKER_REDIS_DB"
const envWorkerRedisPasswd = "WORKER_REDIS_PASSWD"
const envWorkerRedisChannel = "WORKER_REDIS_CHANNEL"

type Worker struct {
	redis *redis.Client
	client *vision.ImageAnnotatorClient
	config *workerConfig
}

type workerConfig struct {
	ctx context.Context
	redis struct{
		channel string
		addr string
		passwd string
		db int
	}
}

type PhotoMetadata struct {
	PhotoId      string `json:"photo_id"      mapstructure:"photo_id"`
	ChatId       int64  `json:"chat_id"       mapstructure:"chat_id"`
	PhotoUrl     string `json:"photo_url"     mapstructure:"photo_url"`
	Caption      string `json:"caption"       mapstructure:"caption"`
	FinalCaption string `json:"final_caption" mapstructure:"final_caption"`
	CaptionRu    string `json:"caption_ru"    mapstructure:"caption_ru"`
	Hashtag      string `json:"hashtag"       mapstructure:"hashtag"`
	HashtagRu    string `json:"hashtag_ru"    mapstructure:"hashtag_ru"`
	StyledUrl    string `json:"styled_url"    mapstructure:"styled_url"`
	Publish      bool   `json:"publish"       mapstructure:"publish"`
	Published    bool   `json:"published"     mapstructure:"published"`
	PublishedUrl string `json:"published_url" mapstructure:"published_url"`
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

	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)

	if err != nil {
		log.Fatalf("[ERROR] Couldn't start Google Vision Image Annotator Client: %s", err)
	}

	worker.client = client
	worker.config.ctx = ctx

	defer ctx.Done()

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
}

func (worker Worker) handleRedis(message *redis.Message) {
	log.Printf("[DEBUG] Got message from redis channel %s: %v",
		worker.config.redis.channel, message)

	var updateMsg PhotoMetadata
	err := json.Unmarshal([]byte(message.Payload), &updateMsg)

	if err != nil {
		log.Printf("[ERROR] Couldn't decode JSON metadata, %s", message.Payload)
	}

	log.Printf("[DEBUG] Got metadata from message %v", updateMsg)

	if len(updateMsg.Hashtag) == 0 {
		res, err := worker.redis.HGetAll(updateMsg.PhotoId).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't hget from redis for ID %s: %s",
				updateMsg.PhotoId, err)
		}
		log.Printf("[DEBUG] Got from redis: %v", res)

		var metaFromRedis PhotoMetadata
		err = mapstructure.WeakDecode(res, &metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't map response from API to metadata struct: %s", err)
			return
		}

		log.Printf("[DEBUG] got metadata from redis: %v", metaFromRedis)

		if len(metaFromRedis.Hashtag) != 0 {
			log.Printf("[INFO] Nothing to do. Already has hashtag: %s",
				metaFromRedis.Hashtag)

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

		hashtag, err := worker.process(metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't get hashtag from API: %s", err)
			return
		}

		metaFromRedis.Hashtag = hashtag

		_, err = worker.redis.HSet(updateMsg.PhotoId, "hashtag", hashtag).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set hashtag in redis for %s: %s",
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

func (worker Worker) process(metadata PhotoMetadata) (string, error) {
	resp, err := getPhoto(metadata.PhotoUrl)

	if err != nil {
		log.Printf("[ERROR] Couldn't get photo: %s", err)
		return "", err
	}

	b := bytes.NewBuffer(make([]byte, 0)) // temporary buffer
	photo := io.TeeReader(resp.Body, b) // returns a reader that writes contents of resp.Body to b

	image, err := vision.NewImageFromReader(photo)

	if err != nil {
		log.Printf("[ERROR] Couldn't read photo: %s", err)
		return "", err
	}

	defer resp.Body.Close() // we're done w/ resp.Body
	resp.Body = ioutil.NopCloser(b) // returns a ReadCloser w/ no-op Close

	labels, err := worker.client.DetectLabels(worker.config.ctx, image, nil, 10)

	if err != nil {
		log.Printf("[ERROR] Couldn't detect image labels: %s", err)
		return "", err
	}

	res := ""

	for _, label := range labels {
		descr := label.Description
		res += "#"
		res += strings.Replace(descr, " ", "_", -1)
		res += " "
	}

	return res, nil
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
