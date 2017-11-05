package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/mitchellh/mapstructure"
	"github.com/nicksnyder/go-i18n/i18n"
	"github.com/spf13/viper"
	"gopkg.in/telegram-bot-api.v4"
)

type Server struct {
	bot *tgbotapi.BotAPI
	redis *redis.Client
	config *serverConfig
}

type serverConfig struct {
	token string
	debug bool
	timeout int
	demoInstaURL string
	redis struct{
		addr string
		passwd string
		channel string
		db int
	}
	sleep int // duration between messages in ms
	chatConfig map[int64]chatConfig
	translation struct{
		en i18n.TranslateFunc
		ru i18n.TranslateFunc
	}
}

type chatConfig struct {
	locale i18n.TranslateFunc
	photoCount int
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

const envTelegramBotToken = "TELEGRAM_BOT_TOKEN"
const envTelegramBotSleep = "TELEGRAM_BOT_SLEEP"
const envTelegramBotDebug = "TELEGRAM_BOT_DEBUG"
const envTelegramDemoInstaURL = "TELEGRAM_DEMO_INSTA_URL"
const envTelegramBotTimeout = "TELEGRAM_BOT_TIMEOUT"
const envTelegramRedisAddr = "TELEGRAM_REDIS_ADDR"
const envTelegramRedisPasswd = "TELEGRAM_REDIS_PASSWD"
const envTelegramRedisChannel = "TELEGRAM_REDIS_CHANNEL"
const envTelegramRedisDb = "TELEGRAM_REDIS_DB"

func main() {
	server := NewServer()
	server.Start()
}

func NewServer() *Server {
	var server Server

	server.config = config()

	bot, err := tgbotapi.NewBotAPI(server.config.token)

	if err != nil {
		panic(fmt.Sprintf("Couldn't create Telegram bot using token '%s': %s",
			server.config.token, err))
	}

	log.Print("[INFO] Telegram server successfully created")

	bot.Debug = server.config.debug

	server.bot = bot

	server.redis = redis.NewClient(&redis.Options{
		Addr: server.config.redis.addr,
		Password: server.config.redis.passwd,
		DB: server.config.redis.db,
	})

	go server.redisSetup()

	return &server
}

func (server Server) redisSetup() {
	pong, err := server.redis.Ping().Result()

	if err != nil {
		log.Printf("[ERROR] Couldn't ping redis server %s", err)
	} else {
		log.Printf("[DEBUG] got pong from redis %v", pong)
	}

	pubsub := server.redis.Subscribe(server.config.redis.channel)
	ch := pubsub.Channel()

	subscr, err := pubsub.ReceiveTimeout(time.Second*time.Duration(10))

	if err != nil {
		log.Printf("[ERROR] Couldn't subscribe to redis channel %s: %s",
			server.config.redis.channel, err)
	}

	log.Printf("[INFO] subscribed to redis channel %s: %v",
		server.config.redis.channel, subscr)

	for message := range ch {
		go func(messageVal *redis.Message){
			server.handleRedis(messageVal)
		}(message)
	}
}

func i18nSetup() (i18n.TranslateFunc, i18n.TranslateFunc) {
	workDir, err := os.Getwd()

	if err != nil {
		log.Printf("[ERROR] Couldn't get working dir: %s", err)
	}

	i18n.MustLoadTranslationFile(workDir + "/i18n/en-us.all.json")
	i18n.MustLoadTranslationFile(workDir + "/i18n/ru-ru.all.json")

	t_en, err := i18n.Tfunc("en-us")

	if err != nil {
		log.Printf("[ERROR] Couldn't create translation function for lang %s", "en-us")
	}

	t_ru, err := i18n.Tfunc("ru-ru")
	if err != nil {
		log.Printf("[ERROR] Couldn't create translation function for lang %s", "ru-ru")
	}

	return t_en, t_ru
}

func config() *serverConfig {
	viper.AutomaticEnv()
	viper.SetDefault(envTelegramBotDebug, false)
	viper.SetDefault(envTelegramBotTimeout, 60)
	viper.SetDefault(envTelegramBotSleep, 300)
	viper.SetDefault(envTelegramDemoInstaURL, "https://instagram.com/nuxdie")
	viper.SetDefault(envTelegramRedisAddr, "localhost:6379")
	viper.SetDefault(envTelegramRedisPasswd, "")
	viper.SetDefault(envTelegramRedisDb, 0)
	viper.SetDefault(envTelegramRedisChannel, "queue")

	if len(viper.GetString(envTelegramBotToken)) == 0 {
		log.Fatal("[FATAL] Please provide a valid Telegram Bot token")
	}

	conf := &serverConfig{
		token: viper.GetString(envTelegramBotToken),
		debug: viper.GetBool(envTelegramBotDebug),
		timeout: viper.GetInt(envTelegramBotTimeout),
		demoInstaURL: viper.GetString(envTelegramDemoInstaURL),
		sleep: viper.GetInt(envTelegramBotSleep),
		chatConfig: make(map[int64]chatConfig),
	}

	t_en, t_ru := i18nSetup()
	conf.translation.en = t_en
	conf.translation.ru = t_ru

	conf.redis.addr = viper.GetString(envTelegramRedisAddr)
	conf.redis.passwd = viper.GetString(envTelegramRedisPasswd)
	conf.redis.channel = viper.GetString(envTelegramRedisChannel)
	conf.redis.db = viper.GetInt(envTelegramRedisDb)

	return conf
}

func (server *Server) Start() {
	u := tgbotapi.NewUpdate(0) // get last updates from offset 0
	u.Timeout = server.config.timeout

	log.Printf("[DEBUG] started listening for telegram updates with timeout %d",
		server.config.timeout)

	updates, err := server.bot.GetUpdatesChan(u)

	if err != nil {
		log.Printf("[ERROR] Couldn't get updates from chan %s: %s", u, err)
	}

	for update := range updates {
		go func (updateVal tgbotapi.Update) {
			server.handleUpdate(updateVal)
		}(update)
	}
}

func (server Server) handleRedis(message *redis.Message) {
	log.Printf("[DEBUG] Got message from redis channel %s: %v",
		server.config.redis.channel, message)

	var updateMsg PhotoMetadata
	err := json.Unmarshal([]byte(message.Payload), &updateMsg)

	if err != nil {
		log.Printf("[ERROR] Couldn't decode JSON metadata, %s", message.Payload)
	}

	res, err := server.redis.HGetAll(updateMsg.PhotoId).Result()

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

	server.checkIfReady(metaFromRedis)
}

func (server Server) checkIfReady(metadata PhotoMetadata) {
	log.Printf("[INFO] cheking metadata from redis: %v", metadata)

	if len(metadata.Hashtag) != 0 &&
	len(metadata.Caption) != 0 &&
	metadata.Publish == false &&
	metadata.Published == false {
		_, err := server.redis.HSet(metadata.PhotoId, "publish", true).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set photo %s for publishing: %s",
				metadata.PhotoId, err)
			return
		}

		metadata.Publish = true

		meta, err := json.Marshal(&metadata)

		if err != nil {
			log.Printf("[ERROR] Couldn't encode JSON: %s", err)
		}
		_, err = server.redis.Publish(server.config.redis.channel, meta).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't publish photo metadata to redis channel: %s",
				server.config.redis.channel, err)
		}

		info := metadata.Caption + "\n.\n.\n.\n" + metadata.Hashtag
		msg := tgbotapi.NewMessage(metadata.ChatId, server.t(metadata.ChatId,
				"all_fields_ready", struct {
				Info string
			}{Info: info}))
		server.bot.Send(msg)
		return
	}

	if metadata.Published {
		log.Printf("[INFO] Published %s.", metadata.PhotoId)
		msg := tgbotapi.NewMessage(metadata.ChatId, server.t(metadata.ChatId,
			"published", struct {
				Url string
			}{Url: metadata.PublishedUrl}))
		server.bot.Send(msg)
	}
}

func (server *Server) handleUpdate(update tgbotapi.Update) {
	log.Printf("[INFO] New update from chat %v @%s: %s",
		update.Message.Chat.ID, update.Message.Chat.UserName, update.Message.Text)

	if len(update.Message.Text) != 0 {
		server.handleText(update)
	}

	if update.Message.Document != nil {
		server.handleDocument(update)
	}

	if update.Message.Photo != nil {
		server.handlePhoto(update)
	}
}

func (server *Server) handleText(update tgbotapi.Update) {
	switch update.Message.Text {
	case "/start":
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID,"greeting", struct {
				Person string
			}{Person: update.Message.Chat.FirstName}))
		server.bot.Send(msg)

		time.Sleep(time.Millisecond * time.Duration(server.config.sleep))
		server.sendIntro1(update)
		time.Sleep(time.Millisecond * time.Duration(server.config.sleep))

		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "switch_locale"))
		msg.ParseMode = "markdown"
		server.bot.Send(msg)
	case "/ru":
		log.Printf("[INFO] Change locale to ru-ru for chat %v", update.Message.Chat.ID)
		server.setLocale(update.Message.Chat.ID, server.config.translation.ru)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "locale_ru"))
		server.bot.Send(msg)

		time.Sleep(time.Millisecond * time.Duration(server.config.sleep))

		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "switch_locale"))
		msg.ParseMode = "markdown"
		server.bot.Send(msg)

		time.Sleep(time.Millisecond * time.Duration(server.config.sleep))
		server.sendIntro1(update)
	case "/en":
		log.Printf("[INFO] Change locale to en-us for chat %v", update.Message.Chat.ID)
		server.setLocale(update.Message.Chat.ID, server.config.translation.en)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "locale_en"))
		server.bot.Send(msg)

		time.Sleep(time.Millisecond * time.Duration(server.config.sleep))

		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "switch_locale"))
		msg.ParseMode = "markdown"
		server.bot.Send(msg)

		time.Sleep(time.Millisecond * time.Duration(server.config.sleep))
		server.sendIntro1(update)
	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "meow"))
		server.bot.Send(msg)
	}
}
func (server *Server) sendIntro1(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		server.t(update.Message.Chat.ID, "step_1_1"))
	server.bot.Send(msg)

	time.Sleep(time.Millisecond * time.Duration(server.config.sleep))

	msg = tgbotapi.NewMessage(update.Message.Chat.ID,
		server.t(update.Message.Chat.ID, "step_1_2", struct {
			DemoInstagram string
		}{DemoInstagram: server.config.demoInstaURL}))
	server.bot.Send(msg)

	time.Sleep(time.Millisecond * time.Duration(server.config.sleep))

	msg = tgbotapi.NewMessage(update.Message.Chat.ID,
		server.t(update.Message.Chat.ID, "step_1_3"))
	server.bot.Send(msg)
}

func (server *Server) handleDocument(update tgbotapi.Update) {
	fileType := update.Message.Document.MimeType
	fileId := update.Message.Document.FileID

	log.Printf("[DEBUG] File type %s ID %s", fileType, fileId)

	if fileType != "image/png" && fileType != "image/jpg" && fileType != "image/jpeg" {
		log.Printf("[WARN] Wrong file type %s received", fileType)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "wrong_file_type", struct {
				Type string
			}{Type: fileType}))
		server.bot.Send(msg)
		return
	}

	photoUrl := server.getFileLink(fileId)
	log.Printf("[INFO] Got photo from Telegram: %s", photoUrl)

	_, err := server.pushPhoto(update.Message.Chat.ID, fileId, photoUrl)

	if err != nil {
		log.Printf("[ERROR] Couldn't publish photo %s: %s", photoUrl, err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "publish_err", struct {
				Error error
			}{Error: err}))
		server.bot.Send(msg)
	}

}

func (server *Server) handlePhoto(update tgbotapi.Update) {
	photos := * update.Message.Photo
	lastPhoto := photos[len(photos) -1] // get the biggest possible photo size
	photoUrl := server.getFileLink(lastPhoto.FileID)

	log.Printf("[INFO] Got photo from Telegram: %s", photoUrl)

	_, err := server.pushPhoto(update.Message.Chat.ID, lastPhoto.FileID, photoUrl)

	if err != nil {
		log.Printf("[ERROR] Couldn't publish photo %s: %s", photoUrl, err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "publish_err", struct {
				Error error
			}{Error: err}))
		server.bot.Send(msg)
	}
}

func (server Server) pushPhoto(chatId int64, photoId, photoUrl string) (int64, error) {
	metadata, err := json.Marshal(&PhotoMetadata{
		ChatId:chatId,
		PhotoUrl:photoUrl,
		PhotoId:photoId,
	})

	log.Printf("[DEBUG] JSON encoded metadata: %s", string(metadata))
	if err != nil {
		log.Printf("[ERROR] Couldn't encode photo metadata to JSON: %s", err)
		return 0, err
	}

	_, err = server.redis.HSet(photoId, "photo_url", photoUrl).Result()

	if err != nil {
		log.Printf("[ERROR] Couldn't hset field %s: %s", "photo_url", err)
	}

	_, err = server.redis.HSet(photoId, "chat_id", chatId).Result()

	if err != nil {
		log.Printf("[ERROR] Couldn't hset field %s: %s", "chat_id", err)
	}


	_, err = server.redis.HSet(photoId, "photo_id", photoId).Result()

	if err != nil {
		log.Printf("[ERROR] Couldn't hset field %s: %s", "photo_id", err)
	}

	res, err := server.redis.Publish(server.config.redis.channel, metadata).Result()

	if err != nil {
		log.Printf("[ERROR] Couldn't publish photo metadata to redis channel: %s",
			server.config.redis.channel, err)
		return 0, err
	}

	return res, nil
}

func (server *Server) getFileLink(fileId string) string {
	file, err := server.bot.GetFile(tgbotapi.FileConfig{FileID: fileId})

	if err != nil {
		log.Printf("[ERROR] Couldn't get file url by id '%s': %s", fileId, err)
	}

	fileUrl := "https://api.telegram.org/file/bot" + server.config.token + "/" + file.FilePath
	return fileUrl
}

func (server Server) setLocale(chatId int64, locale i18n.TranslateFunc) {
	chatConf := server.config.chatConfig[chatId]
	chatConf.locale = locale
	log.Printf("[DEBUG] Set locale %v for chatId %v", locale, chatId)
	server.config.chatConfig[chatId] = chatConf
	// TODO store this in persistent storage
	// TODO Make sure hashtags and caption are translated
}

func (server Server) t(chatId int64, translationID string, args ...interface{}) string {
	chatConf := server.config.chatConfig[chatId]
	tFunc := chatConf.locale // TODO get this from persistent storage

	if tFunc == nil { // set default locale for chatId
		server.setLocale(chatId, server.config.translation.en)
		return server.t(chatId, translationID, args...)
	}

	return tFunc(translationID, args...)
}

// TODO we should use this function in order to generate finalCaption for upload to instagram
func mergeCaptions(caption string, emoji string, hashtags string) string {
	return caption + emoji + "\n.\n.\n.\n" + hashtags
}
