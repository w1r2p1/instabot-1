package main
//TODO implement like caption bot
//TODO use if config says so
/*
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
)

const envWorkerRedisAddr = "WORKER_REDIS_ADDR"
const envWorkerRedisDb = "WORKER_REDIS_DB"
const envWorkerRedisPasswd = "WORKER_REDIS_PASSWD"
const envWorkerRedisChannel = "WORKER_REDIS_CHANNEL"
const envWorkerCaptionApiUrl = "WORKER_CAPTION_URL"
const envWorkerCaptionApiKey = "WORKER_CAPTION_KEY"

type Worker struct {
	redis *redis.Client
	config *workerConfig
}

type workerConfig struct {
	captionApi struct{
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

type PhotoMetadata struct {
	PhotoId      string `json:"photo_id"      mapstructure:"photo_id"`
	ChatId       int64  `json:"chat_id"       mapstructure:"chat_id"`
	PhotoUrl     string `json:"photo_url"     mapstructure:"photo_url"`
	Caption      string `json:"caption"       mapstructure:"caption"`
	CaptionRu    string `json:"caption_ru"    mapstructure:"caption_ru"`
	Hashtag      string `json:"hashtag"       mapstructure:"hashtag"`
	HashtagRu    string `json:"hashtag_ru"    mapstructure:"hashtag_ru"`
	StyledUrl    string `json:"styled_url"    mapstructure:"styled_url"`
	Publish      bool   `json:"publish"       mapstructure:"publish"`
	Published    bool   `json:"published"     mapstructure:"published"`
	PublishedUrl string `json:"published_url" mapstructure:"published_url"`
}

type CaptionApiResponse struct {
	Output string
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

	if len(worker.config.captionApi.key) == 0 {
		log.Fatal("[FATAL] Couldn't create worker due to laking caption api key")
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
	viper.SetDefault(envWorkerCaptionApiUrl, "https://api.deepai.org/api/neuraltalk")
	viper.SetDefault(envWorkerRedisAddr, "localhost:6379")
	viper.SetDefault(envWorkerRedisPasswd, "")
	viper.SetDefault(envWorkerRedisChannel, "queue")
	viper.SetDefault(envWorkerRedisDb, 0)

	conf := &workerConfig{}

	conf.captionApi.url = viper.GetString(envWorkerCaptionApiUrl)
	conf.captionApi.key = viper.GetString(envWorkerCaptionApiKey)

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

	if len(updateMsg.Caption) == 0 {
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

		if len(metaFromRedis.Caption) != 0 {
			log.Printf("[INFO] Nothing to do. Already has caption: %s",
				metaFromRedis.Caption)

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
		caption, err := worker.process(metaFromRedis)

		if err != nil {
			log.Printf("[ERROR] Couldn't get caption from API: %s", err)
			return
		}

		metaFromRedis.Caption = caption

		_, err = worker.redis.HSet(updateMsg.PhotoId, "caption", caption).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set caption in redis for %s: %s",
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
	var postData bytes.Buffer

	resp := getPhoto(metadata.PhotoUrl)

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

	req, err := http.NewRequest("POST", worker.config.captionApi.url, &postData)

	if err != nil {
		log.Printf("[ERROR] Couldn't create http request: %s", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Api-Key", worker.config.captionApi.key)

	client := &http.Client{}

	res, err := client.Do(req)

	if err != nil {
		log.Printf("[ERROR] Couldn't make a request to caption api: %s", err)
	}

	var captionResponse CaptionApiResponse

	err = json.NewDecoder(res.Body).Decode(&captionResponse)

	if err != nil {
		log.Printf("[ERROR] Couldn't read response from api: %s", err)
	}

	defer res.Body.Close()

	var captionErr error

	if len(captionResponse.Err) != 0 {
		captionErr = errors.New(captionResponse.Err)
	} else {
		captionErr = nil
	}

	return captionResponse.Output, captionErr
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

func stylize(resp * http.Response, filter string) * http.Response {
		styleApiUri := os.Getenv("STYLE_SERVER_URL")

		if len(styleApiUri) == 0 {
			panic("Please provide style API uri")
		}

		var postData bytes.Buffer
		w := multipart.NewWriter(&postData)
		fw, err := w.CreateFormFile("file", "file.jpg")

		if err != nil {
			panic(fmt.Sprintf("Couldn't create form file %s", err))
		}

		if _, err = io.Copy(fw, resp.Body); err != nil {
			panic(fmt.Sprintf("Couldn't copy file to form dest %s", err))
		}

		if fw, err = w.CreateFormField("filter"); err != nil {
			panic(fmt.Sprintf("Couldn't create a filter field on form data", err))
		}

		if _, err = fw.Write([]byte(filter)); err != nil {
			panic(fmt.Sprintf("Couldn't write filter to field on form data", err))
		}

		w.Close() // So the terminating boundary would be there in place

		req, err := http.NewRequest("POST", styleApiUri, &postData)

		if err != nil {
			panic(fmt.Sprintf("Couldn't create a new http request"))
		}
		// setting correct headers to calculate the post body boundary
		req.Header.Set("Content-Type", w.FormDataContentType())

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			panic(fmt.Sprintf("Error while doing a request %s: %s", req, res))
		}

		return res
}

func randomFilter() string {
	//TODO select style filter based on some scoring
	//for example take this approach from MIT MemNet:
	//http://memorability.csail.mit.edu/download.html

rand.Seed(time.Now().Unix())
filters := []string{
"la_muse",
"rain_princess",
"scream",
"udnie",
"wave",
"wreck",
"dora_marr",
}
i := rand.Int() % len(filters)
return filters[i]
}

*/