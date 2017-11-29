# Caption Worker
gets caption for a photo

## Setup

### Redis
This environment variables play major parts in worker Redis connection:
````bash
WORKER_REDIS_ADDR=localhost:6379
WORKER_REDIS_DB=0
WORKER_REDIS_PASSWD=""
WORKER_REDIS_CHANNEL="queue"
````

### Photo caption
to get a nice caption for the photo, use something like [deepai API](https://deepai.org/machine-learning-model/neuraltalk)
````bash
WORKER_CAPTION_URL=https://api.deepai.org/api/neuraltalk
WORKER_CAPTION_KEY=fffffff-0123-4567-8901-fffffffffffff
````