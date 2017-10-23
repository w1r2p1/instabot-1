package main

import (
	"os"
	"io"
	"fmt"
	"net/url"
	"net/http"

	"github.com/ahmdrz/goinsta"
	"github.com/ahmdrz/goinsta/response"
)

func main() {

	insta := login()

	photoFileReader := getPhoto()
	photoCaption := "TODO #hashtags" // TODO: proper hash tags from Vision API

	upload(insta, photoFileReader, photoCaption)

	defer photoFileReader.Close()
	defer insta.Logout()
}

func getPhoto() io.ReadCloser {
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

	return resp.Body
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
		panic(err)
	}

	fmt.Printf("Upload status:%s MediaID: %s", uploadPhotoResponse.Status, uploadPhotoResponse.Media.ID)

	return uploadPhotoResponse
}