package main

import (
	"os"
	"fmt"
	"github.com/ahmdrz/goinsta"
	"github.com/ahmdrz/goinsta/response"
)

func main() {
	username := os.Getenv("INSTAGRAM_USERNAME")
	password := os.Getenv("INSTAGRAM_PASSWORD")

	if len(username) * len(password) == 0 {
		panic("No Instagram username and/or password provided.")
	}

	insta := goinsta.New(username, password)

	if err := insta.Login(); err != nil {
		panic(err)
	}

	if len(os.Args) == 1 {
		panic("Please provide a photo path.")
	}

	photoFile := os.Args[1]
	photoCaption := "TODO #hashtags"

	upload(insta, photoFile, photoCaption)

	defer insta.Logout()
}

func upload(insta * goinsta.Instagram, path string, caption string) response.UploadPhotoResponse {
	quality := 87
	uploadId := insta.NewUploadID()
	filterType := goinsta.Filter_Normat

	response, err := insta.UploadPhoto(path, caption, uploadId, quality, filterType)

	if err != nil {
		panic(err)
	}

	fmt.Printf("Upload status:%s MediaID: %s", response.Status, response.Media.ID)

	return response
}