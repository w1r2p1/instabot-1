#!/usr/bin/env bash
GOOS=linux GOARCH=amd64 go build -v -o build/app
ssh -t box "sudo systemctl stop instabot"
scp ./build/app box:~/w/InstaBot/
ssh -t box "sudo systemctl start instabot"
