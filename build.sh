#!/usr/bin/env bash
for WORKER in telegram instagram caption hashtag
do
  cd ${WORKER}
  echo ""
  echo "Building ${WORKER}"
  cp ../ca-certificates.crt .
  CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -v -o build/main .
  cd ..
done

cp key.json hashtag/