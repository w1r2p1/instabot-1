# Telegram Bot Server
gets hashtags for a photo from Google Vision API

## Setup

### Telegram bot
````bash
TELEGRAM_BOT_TOKEN=123456789:FSw4TQw4gwaRARDfasdfaW$R@qrh9jhu
TELEGRAM_BOT_SLEEP=300
TELEGRAM_BOT_DEBUG=false
TELEGRAM_DEMO_INSTA_URL=https://instagram.com/nuxdie
TELEGRAM_DEMO_LANDING_URL=https://instabeat.ml/?utm_source=telegram
TELEGRAM_BOT_TIMEOUT=60
````

### Redis
This environment variables play major parts in worker Redis connection:
````bash
TELEGRAM_REDIS_ADDR==localhost:6379
TELEGRAM_REDIS_DB=0=
TELEGRAM_REDIS_PASSWD==""
TELEGRAM_REDIS_CHANNEL=="queue"
````

### Google Vision API 
credentials for adding appropriate #hashtags,
see [this guide to setup](https://cloud.google.com/docs/authentication/getting-started)
````bash
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service/account/file.json
````