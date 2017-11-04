package telegram

import (
	"github.com/spf13/viper"
	"gopkg.in/telegram-bot-api.v4"
	"fmt"
	"log"
	"github.com/nicksnyder/go-i18n/i18n"
	"os"
	"time"
)

type Server struct {
	bot *tgbotapi.BotAPI
	config *serverConfig
}

type serverConfig struct {
	token string
	debug bool
	timeout int
	demoInstaURL string
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

type publishSuccess string

const envTelegramBotToken = "TELEGRAM_BOT_TOKEN"
const envTelegramBotSleep = "TELEGRAM_BOT_SLEEP"
const envTelegramBotDebug = "TELEGRAM_BOT_DEBUG"
const envTelegramDemoInstaURL = "TELEGRAM_DEMO_INSTA_URL"
const envTelegramBotTimeout = "TELEGRAM_BOT_TIMEOUT"

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

	return &server
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
		server.handleUpdate(update)
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

	_, err := server.publishPhoto(photoUrl)

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

	_, err := server.publishPhoto(photoUrl)

	if err != nil {
		log.Printf("[ERROR] Couldn't publish photo %s: %s", photoUrl, err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			server.t(update.Message.Chat.ID, "publish_err", struct {
				Error error
			}{Error: err}))
		server.bot.Send(msg)
	}
}

func (server Server) publishPhoto(photoUrl string) (publishSuccess, error) {
	//TODO put this update to some queue

	return "ok", nil
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