# Emoji Worker
gets emoji for a photo caption

## Setup

### Redis
This environment variables play major parts in worker Redis connection:
````bash
WORKER_REDIS_ADDR=localhost:6379
WORKER_REDIS_DB=0
WORKER_REDIS_PASSWD=""
WORKER_REDIS_CHANNEL="queue"
````

*TODO emoji api setup

### Emoji
to have a nice emoji icons in photo caption, use something like [this serverless API](https://github.com/nuxdie/emojify)
````bash
EMOJI_API_URL=https://wt-01234567890-0.sandbox.auth0-extend.com/emojify
````
