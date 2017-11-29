# Hashtag Worker
gets hashtags for a photo from Google Vision API

## Setup

### Redis
This environment variables play major parts in worker Redis connection:
````bash
WORKER_REDIS_ADDR=localhost:6379
WORKER_REDIS_DB=0
WORKER_REDIS_PASSWD=""
WORKER_REDIS_CHANNEL="queue"
````

### Google Vision API 
credentials for adding appropriate #hashtags,
see [this guide to setup](https://cloud.google.com/docs/authentication/getting-started)
````bash
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service/account/file.json
````