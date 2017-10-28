package main

import (
	"os"
	"io"
	"io/ioutil"
	"fmt"
	"bytes"
	"net/url"
	"net/http"
	"strings"
	"context"

	"github.com/ahmdrz/goinsta"
	"github.com/ahmdrz/goinsta/response"

	vision "cloud.google.com/go/vision/apiv1"

	"gopkg.in/telegram-bot-api.v4"
)

func main() {
	startTelegramBotServer()
}

func getFileAndUpload(uri string) response.UploadPhotoResponse {
	insta := loginInstagram()

	resp := getPhoto(uri)
	photoCaption := getHashtags(resp)
	// TODO stylize photo using github.com/nuxdie/fast-style-transfer
	uploadPhotoResponse := upload(insta, resp.Body, photoCaption)
	disableComments(insta, uploadPhotoResponse)

	defer resp.Body.Close()
	defer insta.Logout()

	return uploadPhotoResponse
}

func startTelegramBotServer() * tgbotapi.BotAPI {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	botDebug := os.Getenv("DEBUG_TELEGRAM_BOT")

	if len(botToken) == 0 {
		panic("Please provide a valid Telegram Bot token")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)

	if err != nil {
		panic(fmt.Sprintf("Couldn't create Telegram bot using token '%s': %s", botToken, err))
	}

	bot.Debug = bool(len(botDebug) != 0)

	webhookEnabled := os.Getenv("WEBHOOK_MODE")

	if len(webhookEnabled) != 0 {
		setWebhook(bot)
	} else {
		longPollUpdates(bot)
	}

	return bot
}

func longPollUpdates(bot * tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0) // get last updates from offset 0
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	if err != nil {
		panic(fmt.Sprintf("Couldn't get updates from chan %s: %s", u, err))
	}

	for update := range updates {
		handleUpdate(bot, update)
	}
}

func setWebhook(bot * tgbotapi.BotAPI) {
	// TODO this needs to be battle-tested!
	webhookUrl := os.Getenv("SERVER_BASE_URL")
	certfile := os.Getenv("CERT_FILE")

	if len(webhookUrl) * len(certfile) == 0 {
		panic(fmt.Sprintf("Please provide valid webhook url '%s' and certfile '%s'", webhookUrl, certfile))
	}

	_, err := bot.SetWebhook(tgbotapi.NewWebhookWithCert(webhookUrl, certfile))

	if err != nil {
		panic(fmt.Sprintf("Unable to set webhook url '%s' for bot: %s", webhookUrl, err))
	}

	updates := bot.ListenForWebhook("/")

	keyfile := os.Getenv("KEY_FILE")

	if len(keyfile) == 0 {
		panic("Please provide valid keyfile")
	}

	go http.ListenAndServeTLS("0.0.0.0:8433", certfile, keyfile, nil)

	for update := range updates {
		handleUpdate(bot, update)
	}
}

func handleUpdate(bot * tgbotapi.BotAPI, update tgbotapi.Update) {
	photos := * update.Message.Photo
	lastPhoto := photos[len(photos) -1] // get the biggest possible photo size
	photoId := lastPhoto.FileID

	photo, err := bot.GetFile(tgbotapi.FileConfig{FileID: photoId})

	if err != nil {
		panic(fmt.Sprintf("Couldn't get file url by id '%s': %s", photoId, err))
	}

	photoUrl := "https://api.telegram.org/file/bot" + bot.Token + "/" + photo.FilePath
	uploadPhotoResponse := getFileAndUpload(photoUrl)

	responseMessage := fmt.Sprintf("Upload status:%s MediaID: %s", uploadPhotoResponse.Status, uploadPhotoResponse.Media.ID)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, responseMessage)

	bot.Send(msg)
}

func disableComments(insta *goinsta.Instagram, uploadPhotoResponse response.UploadPhotoResponse) {
	_, err := insta.DisableComments(uploadPhotoResponse.Media.ID)

	if err != nil {
		panic(fmt.Sprintf("Error trying to disable comments for mediaId %s: %s", uploadPhotoResponse.Media.ID, err))
	}
}

func getHashtags(resp * http.Response) string {
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)

	if err != nil {
		panic(fmt.Sprintf("Couldn't start Google Vision Image Annotator Client: %s", err))
	}

	b := bytes.NewBuffer(make([]byte, 0)) // temporary buffer
	photo := io.TeeReader(resp.Body, b) // returns a reader that writes contents of resp.Body to b

	image, err := vision.NewImageFromReader(photo)

	if err != nil {
		panic(fmt.Sprintf("Couldn't read photo: %s", err))
	}

	defer resp.Body.Close() // we're done w/ resp.Body
	resp.Body = ioutil.NopCloser(b) // returns a ReadCloser w/ no-op Close

	labels, err := client.DetectLabels(ctx, image, nil, 10)

	if err != nil {
		panic(fmt.Sprintf("Couldn't detect image labels: %s", err))
	}

	defer ctx.Done()

	res := ""

	for _, label := range labels {
		descr := label.Description
		res += "#"
		res += strings.Replace(descr, " ", "_", -1)
		res += " "
	}

	return res
}

func getPhoto(uri string) * http.Response {
	if len(uri) == 0 {
		panic("Please provide a photo url.")
	}

	parsed, err := url.Parse(uri)

	if parsed.Host == "" || err != nil {
		panic(fmt.Sprintf("Incorrect photo url provided: %s", uri))
	}

	resp, err := http.Get(uri)

	if err != nil {
		panic(fmt.Sprintf("Could not get the photo by %s", uri))
	}

	return resp
}

func loginInstagram() * goinsta.Instagram {
	username := os.Getenv("INSTAGRAM_USERNAME")
	password := os.Getenv("INSTAGRAM_PASSWORD")

	if len(username) * len(password) == 0 {
		panic("No Instagram username and/or password provided.")
	}

	insta := goinsta.New(username, password)


	if err := insta.Login(); err != nil {
		panic(err)
	}

	return insta
}

func upload(insta * goinsta.Instagram, photo io.ReadCloser, caption string) response.UploadPhotoResponse {
	quality := 87
	uploadId := insta.NewUploadID()
	filterType := goinsta.Filter_Valencia // TODO select filter randomly or based on some scoring

	uploadPhotoResponse, err := insta.UploadPhotoFromReader(photo, caption, uploadId, quality, filterType)

	if err != nil {
		panic(fmt.Sprintf("Couldn't upload photo to instagram: %s", err))
	}

	return uploadPhotoResponse
}