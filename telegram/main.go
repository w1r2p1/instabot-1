package telegram

import (
	"github.com/spf13/viper"
	"gopkg.in/telegram-bot-api.v4"
	"fmt"
	"log"
	"github.com/nicksnyder/go-i18n/i18n"
	"os"
)

type Server struct {
	bot *tgbotapi.BotAPI
	config *serverConfig
	t i18n.TranslateFunc
}

type serverConfig struct {
	token string
	debug bool
	timeout int
	translation struct{
		en i18n.TranslateFunc
		ru i18n.TranslateFunc
	}
}

const envTelegramBotToken = "TELEGRAM_BOT_TOKEN"
const envTelegramBotDebug = "TELEGRAM_BOT_DEBUG"
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
	server.t = server.config.translation.en // default locale

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

	if len(viper.GetString(envTelegramBotToken)) == 0 {
		log.Fatal("[FATAL] Please provide a valid Telegram Bot token")
	}

	conf := &serverConfig{
		token: viper.GetString(envTelegramBotToken),
		debug: viper.GetBool(envTelegramBotDebug),
		timeout: viper.GetInt(envTelegramBotTimeout),
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

	if update.Message.Text == "/start" {
		server.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf(server.t("greeting", struct {
			Person string
		}{Person:update.Message.Chat.FirstName}))))
	}
}
