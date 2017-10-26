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
)

func main() {

	insta := login()

	resp := getPhoto()
	photoCaption := getHashtags(resp)

	upload(insta, resp.Body, photoCaption)
	/* TODO: turn off commenting
	to do that i'll need to add media/configure route
	[here](github.com/ahmdrz/goinsta/goinsta.go:841)
	and to be sure i need to root my phone or use an
	emulator to get instagram running through charles
	[proxy](http://eliasbagley.github.io/reverseengineering/2016/12/02/reverse-engineering-instagram-api.html)
	for some inspiration could be found in js version
	of [this api](https://github.com/huttarichard/instagram-private-api/blob/4f2c33dc177ac12ac555af4cfb9db15d98e882f2/client/v1/media.js#L215)
	*/
	defer resp.Body.Close()
	defer insta.Logout()
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

func getPhoto() * http.Response {
	if len(os.Args) == 1 {
		panic("Please provide a photo url.")
	}

	uri := os.Args[1]
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

func login() * goinsta.Instagram {
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
	filterType := goinsta.Filter_Normat

	uploadPhotoResponse, err := insta.UploadPhotoFromReader(photo, caption, uploadId, quality, filterType)

	if err != nil {
		panic(fmt.Sprintf("Couldn't upload photo to instagram: %s", err))
	}

	fmt.Printf("Upload status:%s MediaID: %s", uploadPhotoResponse.Status, uploadPhotoResponse.Media.ID)

	return uploadPhotoResponse
}