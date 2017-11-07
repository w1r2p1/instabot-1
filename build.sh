#!/usr/bin/env bash
pushd telegram
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
popd

pushd instagram
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
popd

pushd caption
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
popd

pushd hashtag
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
popd

docker-compose up --build