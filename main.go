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
	"mime/multipart"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/ahmdrz/goinsta"
	"github.com/ahmdrz/goinsta/response"

	vision "cloud.google.com/go/vision/apiv1"

	"gopkg.in/telegram-bot-api.v4"

)

func main() {
	tgserver := telegram.NewServer()
	tgserver.Start()
}

func getFileAndUpload(uri string, bot * tgbotapi.BotAPI, update tgbotapi.Update) response.UploadPhotoResponse {
	insta := loginInstagram()
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "ℹ️ logged in to Instagram"))

	resp := getPhoto(uri)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ got photo from uri: %s", uri)))

	// TODO parse and use geotags from photo

	captionEmoji := getEmoji(photoCaption)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ got some emojis from caption: %s", captionEmoji)))

	photoHashtags := getHashtags(resp)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ synthesized hashtags: %s", photoHashtags)))

	finalCaption := mergeCaptions(photoCaption, captionEmoji, photoHashtags)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ the final caption would be like: %s", finalCaption)))

	filter := randomFilter()
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ picked up filter: %s", filter)))

	styledPhoto := stylize(resp, filter)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ applied style transfer")))

	uploadPhotoResponse := upload(insta, styledPhoto.Body, finalCaption)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ uploaded to Instagram")))

	disableComments(insta, uploadPhotoResponse)
	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ disabled comments")))

	defer styledPhoto.Body.Close()
	defer resp.Body.Close()
	defer insta.Logout()

	bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ logged out from Instagram")))

	return uploadPhotoResponse
}

func getEmoji(text string) string {
	emojiApiUrl := os.Getenv("EMOJI_API_URL")

	if len(emojiApiUrl) == 0 {
		panic("Please provide a valid emoji api url")
	}

	reqUrl := emojiApiUrl + "?text=" + url.QueryEscape(text)
	res, err := http.Get(reqUrl)

	if err != nil || res.StatusCode != 200 {
		panic(fmt.Sprintf("Error while doing a request: %s, %s", err, res))
	}

	emojis, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(fmt.Sprintf("Couldn't read response: %s", err))
	}

	return string(emojis)
}

func mergeCaptions(caption string, emoji string, hashtags string) string {
	return caption + emoji + "\n.\n.\n.\n" + hashtags
}

func randomFilter() string {
	/* TODO select style filter based on some scoring
	for example take this approach from MIT MemNet:
	http://memorability.csail.mit.edu/download.html
	*/
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
	filterType := goinsta.Filter_Valencia

	uploadPhotoResponse, err := insta.UploadPhotoFromReader(photo, caption, uploadId, quality, filterType)

	if err != nil {
		panic(fmt.Sprintf("Couldn't upload photo to instagram: %s", err))
	}

	return uploadPhotoResponse
}