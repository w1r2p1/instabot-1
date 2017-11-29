# Style transfer Worker
apply style transfer to a photo

## Setup

### Redis
This environment variables play major parts in worker Redis connection:
````bash
WORKER_REDIS_ADDR=localhost:6379
WORKER_REDIS_DB=0
WORKER_REDIS_PASSWD=""
WORKER_REDIS_CHANNEL="queue"
````

*TODO add style api info

### Style transfer
in order to get image stylized, use [this server](https://github.com/nuxdie/fast-style-transfer)
 as a reference implementation:
````bash
STYLE_SERVER_URL=https://example.com/upload
````