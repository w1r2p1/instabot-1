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
	"gitlab.com/nuxdie/instabot/metadata"
	"github.com/hashicorp/logutils"
	"strconv"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Server struct {
	bot *tgbotapi.BotAPI
	redis *redis.Client
	config *serverConfig
	mongo *mgo.Session
}

type serverConfig struct {
	token string
	debug bool
	timeout int
	demoInstaURL string
	landingUrl string
	mongo struct{
		url string
		dbName string
	}
	redis struct{
		addr string
		passwd string
		channel string
		db int
	}
	sleep int // duration between messages in ms
	chatConfig map[int64]ChatConfig
	translation map[string]i18n.TranslateFunc
}

type ChatConfig struct {
	ID bson.ObjectId `bson:"_id,omitempty"`
	chatId int64 `bson:"chat_id"`
	locale string `bson:"locale"`
	photoCount int `bson:"photo_count"`
}

const envLogLevel = "LOG_LEVEL"
const envTelegramBotToken = "TELEGRAM_BOT_TOKEN"
const envTelegramBotSleep = "TELEGRAM_BOT_SLEEP"
const envTelegramBotDebug = "TELEGRAM_BOT_DEBUG"
const envTelegramDemoInstaURL = "TELEGRAM_DEMO_INSTA_URL"
const envTelegramDemoLandingUrl = "TELEGRAM_DEMO_LANDING_URL"
const envTelegramBotTimeout = "TELEGRAM_BOT_TIMEOUT"
const envTelegramMongoUrl = "TELEGRAM_MONGO_URL"
const envTelegramMongoDbName = "TELEGRAM_MONGO_DB_NAME"
const envTelegramRedisAddr = "TELEGRAM_REDIS_ADDR"
const envTelegramRedisPasswd = "TELEGRAM_REDIS_PASSWD"
const envTelegramRedisChannel = "TELEGRAM_REDIS_CHANNEL"
const envTelegramRedisDb = "TELEGRAM_REDIS_DB"

const mongoSettingsCollectionName = "settings"

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
	go server.mongoConnect()

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
		log.Fatalf("[ERROR] Couldn't subscribe to redis channel %s: %s",
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

	tEn, err := i18n.Tfunc("en-us")

	if err != nil {
		log.Printf("[ERROR] Couldn't create translation function for lang %s", "en-us")
	}

	tRu, err := i18n.Tfunc("ru-ru")
	if err != nil {
		log.Printf("[ERROR] Couldn't create translation function for lang %s", "ru-ru")
	}

	return tEn, tRu
}

func config() *serverConfig {
	viper.AutomaticEnv()
	viper.SetDefault(envTelegramBotDebug, false)
	viper.SetDefault(envTelegramBotTimeout, 60)
	viper.SetDefault(envTelegramBotSleep, 300)
	viper.SetDefault(envTelegramDemoInstaURL, "https://instagram.com/instabeat7374")
	viper.SetDefault(envTelegramDemoLandingUrl, "https://instabeat.ml/?utm_source=telegram")
	viper.SetDefault(envTelegramMongoUrl, "localhost")
	viper.SetDefault(envTelegramMongoDbName, "instabot")
	viper.SetDefault(envTelegramRedisAddr, "localhost:6379")
	viper.SetDefault(envTelegramRedisPasswd, "")
	viper.SetDefault(envTelegramRedisDb, 0)
	viper.SetDefault(envTelegramRedisChannel, "queue")
	viper.SetDefault(envLogLevel, "WARN")

	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"},
		MinLevel: logutils.LogLevel(viper.GetString(envLogLevel)),
		Writer: os.Stderr,
	}
	log.SetOutput(filter)

	if len(viper.GetString(envTelegramBotToken)) == 0 {
		log.Fatal("[FATAL] Please provide a valid Telegram Bot token")
	}

	conf := &serverConfig{
		token: viper.GetString(envTelegramBotToken),
		debug: viper.GetBool(envTelegramBotDebug),
		timeout: viper.GetInt(envTelegramBotTimeout),
		demoInstaURL: viper.GetString(envTelegramDemoInstaURL),
		landingUrl: viper.GetString(envTelegramDemoLandingUrl),
		sleep: viper.GetInt(envTelegramBotSleep),
		chatConfig: make(map[int64]ChatConfig),
	}

	tEn, tRu := i18nSetup()
	conf.translation = make(map[string]i18n.TranslateFunc)
	conf.translation["en"] = tEn
	conf.translation["ru"] = tRu

	conf.redis.addr = viper.GetString(envTelegramRedisAddr)
	conf.redis.passwd = viper.GetString(envTelegramRedisPasswd)
	conf.redis.channel = viper.GetString(envTelegramRedisChannel)
	conf.redis.db = viper.GetInt(envTelegramRedisDb)

	conf.mongo.url = viper.GetString(envTelegramMongoUrl)
	conf.mongo.dbName = viper.GetString(envTelegramMongoDbName)

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

	var updateMsg metadata.PhotoMetadata
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

	var metaFromRedis metadata.PhotoMetadata
	err = mapstructure.WeakDecode(res, &metaFromRedis)

	if err != nil {
		log.Printf("[ERROR] Couldn't map response from API to metadata struct: %s", err)
		return
	}

	server.checkIfReady(metaFromRedis)
}

func (server Server) checkIfReady(photoMetadata metadata.PhotoMetadata) {
	log.Printf("[DEBUG] cheking metadata from redis: %v", photoMetadata)
	currentChatConfig := server.config.chatConfig[photoMetadata.ChatId]

	if len(photoMetadata.Hashtag) != 0 &&
	len(photoMetadata.Caption) != 0 &&
	photoMetadata.NSFWChecked &&
	!photoMetadata.NSFW &&
	photoMetadata.Publish == false &&
	photoMetadata.Published == false &&
	currentChatConfig.photoCount < 3 {
		_, err := server.redis.HSet(photoMetadata.PhotoId, "publish", true).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set photo %s for publishing: %s",
				photoMetadata.PhotoId, err)
			return
		}

		info := server.mergeCaptions(photoMetadata.Caption, photoMetadata.Hashtag)

		_, err = server.redis.HSet(photoMetadata.PhotoId, "final_caption", info).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't set photo %s final_caption: %s",
				photoMetadata.PhotoId, err)
			return
		}

		photoMetadata.FinalCaption = info

		photoMetadata.Publish = true

		meta, err := json.Marshal(&photoMetadata)

		if err != nil {
			log.Printf("[ERROR] Couldn't encode JSON: %s", err)
		}
		_, err = server.redis.Publish(server.config.redis.channel, meta).Result()

		if err != nil {
			log.Printf("[ERROR] Couldn't publish photo metadata to redis channel %s: %s",
				server.config.redis.channel, err)
		}

		msg := tgbotapi.NewMessage(photoMetadata.ChatId, server.t(photoMetadata.ChatId,
				"all_fields_ready", struct {
				Info string
			}{Info: info}))
		server.bot.Send(msg)
		return
	}

	if photoMetadata.Published {
		log.Printf("[INFO] Published %s.", photoMetadata.PhotoId)

		if currentChatConfig.photoCount == 0 {
			currentChatConfig.photoCount = 1
		} else {
			currentChatConfig.photoCount++
		}
		server.config.chatConfig[photoMetadata.ChatId] = currentChatConfig

		msg := tgbotapi.NewMessage(photoMetadata.ChatId, server.t(photoMetadata.ChatId,
			"published", struct {
				Url string
			}{Url: photoMetadata.PublishedUrl}))
		server.bot.Send(msg)
	}

	if photoMetadata.NSFWChecked && photoMetadata.NSFW {
		log.Printf("[INFO] NSFW detected %s! chatId: %s",
			photoMetadata.PhotoId, photoMetadata.ChatId)
		msg := tgbotapi.NewMessage(photoMetadata.ChatId, server.t(photoMetadata.ChatId,
			"nsfw_detected"))
		server.bot.Send(msg)
	}
}

func (server *Server) handleUpdate(update tgbotapi.Update) {
	log.Printf("[INFO] New update from chat %v @%s: %s",
		update.Message.Chat.ID, update.Message.Chat.UserName, update.Message.Text)

	if len(update.Message.Text) != 0 {
		server.handleText(update)
	}

	if update.Message.Document != nil || update.Message.Photo != nil {
		currentChatConfig := server.config.chatConfig[update.Message.Chat.ID]

		if currentChatConfig.photoCount > 3 {
			landingUrl := server.config.landingUrl+"&chat_id="+strconv.Itoa(int(update.Message.Chat.ID))

			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				server.t(update.Message.Chat.ID, "demo_end_1"))
			server.bot.Send(msg)

			time.Sleep(time.Millisecond * time.Duration(server.config.sleep))

			msg = tgbotapi.NewMessage(update.Message.Chat.ID,
				server.t(update.Message.Chat.ID, "demo_end_2", &struct {
					LandingUrl string
				}{LandingUrl: landingUrl}))
			server.bot.Send(msg)
			return
		}

		if update.Message.Document != nil {
			server.handleDocument(update)
		}

		if update.Message.Photo != nil {
			server.handlePhoto(update)
		}
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
		server.setLocale(update.Message.Chat.ID, "ru")

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
		server.setLocale(update.Message.Chat.ID, "en")

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

	if fileType != "image/jpg" && fileType != "image/jpeg" {
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
	photoMetadata, err := json.Marshal(&metadata.PhotoMetadata{
		ChatId:chatId,
		PhotoUrl:photoUrl,
		PhotoId:photoId,
	})

	log.Printf("[DEBUG] JSON encoded metadata: %s", string(photoMetadata))
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

	res, err := server.redis.Publish(server.config.redis.channel, photoMetadata).Result()

	if err != nil {
		log.Printf("[ERROR] Couldn't publish photo metadata to redis channel %s: %s",
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

func (server Server) setLocale(chatId int64, locale string) {
	chatConf := server.config.chatConfig[chatId]
	chatConf.locale = locale
	log.Printf("[DEBUG] Set locale %v for chatId %v", locale, chatId)
	server.config.chatConfig[chatId] = chatConf

	go server.saveChatConfig(chatId)
	// TODO Make sure hashtags and caption are translated
}

func (server Server) saveChatConfig(chatId int64) error {
	chatConf := server.config.chatConfig[chatId]

	chatConf.chatId = chatId

	log.Printf("[DEBUG] Saving chat config to mongo for %s: %v", chatId, chatConf)

	session, err := mgo.Dial(server.config.mongo.url)

	if err != nil {
		log.Fatalf("Couldn't connect to mongo, %s", err)
		return err
	}

	// FIXME this doesn't work (not setting anything in db)
	c := session.DB(server.config.mongo.dbName).C(mongoSettingsCollectionName)
	err = c.Insert(chatConf)

	defer session.Close()

	if err != nil {
		log.Printf("[ERROR] Couldn't set chat config for %s: %s", chatId, err)
		return err
	}

	return nil
}

func (server Server) getChatLocale(chatId int64) (string, error) {
	session, err := mgo.Dial(server.config.mongo.url)

	if err != nil {
		log.Fatalf("Couldn't connect to mongo, %s", err)
		return "", err
	}

	c := session.DB(server.config.mongo.dbName).C(mongoSettingsCollectionName)

	var result ChatConfig
	err = c.Find(bson.M{"chatId":strconv.Itoa(int(chatId))}).One(&result)

	defer session.Close()

	if err != nil {
		log.Printf("[ERROR] Couldn't find config for chat %s: %s", chatId, err)
		return "", err
	}

	log.Printf("[DEBUG] Found locale for chat %s: %s", chatId, result.locale)

	return result.locale, nil
}

func (server Server) mongoConnect() error {
	log.Printf("[DEBUG] mongo url: %s", server.config.mongo.url)

	session, err := mgo.Dial(server.config.mongo.url)

	if err != nil {
		log.Fatalf("Couldn't connect to mongo, %s", err)
		return err
	}

	log.Printf("[INFO] Connected to mongo!")

	//defer session.Close()

	server.mongo = session

	return nil
}

func (server Server) t(chatId int64, translationID string, args ...interface{}) string {
	chatConf := server.config.chatConfig[chatId]
	localeStr := chatConf.locale

	if localeStr == "" {
		log.Printf("[DEBUG] Trying to get locale from mongo for %s before setting default en",
			chatId)

		localeFromMongo, err := server.getChatLocale(chatId)

		if err != nil {
			log.Printf("[ERROR] Couldn't get locale for chat, %s: %s", chatId, err)
		}

		if localeFromMongo == "" {
			log.Printf("[DEBUG] Couldn't find locale for %s, setting default, en", chatId)
			server.setLocale(chatId, "en")
		} else {
			server.setLocale(chatId, localeFromMongo)
		}
		return server.t(chatId, translationID, args...)
	}

	tFunc := server.config.translation[localeStr]
	return tFunc(translationID, args...)
}

func (server Server) mergeCaptions(caption string, hashtags string) string {
	return caption + "\n.\n.\n.\n" + hashtags
}