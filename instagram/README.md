# Instagram Worker
Publish photo to Instagram

## Setup

### Redis
This environment variables play major parts in worker Redis connection:
````bash
WORKER_REDIS_ADDR=localhost:6379
WORKER_REDIS_DB=0
WORKER_REDIS_PASSWD=""
WORKER_REDIS_CHANNEL="queue"
````

### Instagram 
credentials for an account to post to
````bash
WORKER_INSTAGRAM_USERNAME=username
WORKER_INSTAGRAM_PASSWORD=passw0rd
````