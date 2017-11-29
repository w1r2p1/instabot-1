# NSFW detector
This worker uses [NSFW Detector API](https://api.deepai.org/api/nsfw-detector) to
make sure, we're not pasting something that's not allowed by Instagram rules.


## Setup

### Redis
This environment variables play major parts in worker Redis connection:
````bash
WORKER_REDIS_ADDR=localhost:6379
WORKER_REDIS_DB=0
WORKER_REDIS_PASSWD=""
WORKER_REDIS_CHANNEL="queue"
````

### NSFW API
Same as other workers, here's a list of relevant ENV variables:
````bash
WORKER_NSFW_API_URL=https://api.deepai.org/api/nsfw-detector
WORKER_NSFW_API_KEY=<api key>
````