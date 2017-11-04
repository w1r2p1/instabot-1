# InstaBot
## Project goal
The aim of this project is to build a telegram bot, that accepts an image as a file or photo and uploads it to
Instagram. It also adds appropriate hashtags and geotags based on image data. Video and gallery support could be added
at later stages of the project.

## Setup
In order to get things working user needs to provide these environment variables:

### Instagram 
credentials for an account to post to
````bash
INSTAGRAM_USERNAME=username
INSTAGRAM_PASSWORD=passw0rd
````

### Google Vision API 
credentials for adding appropriate #hashtags,
see [this guide to setup](https://cloud.google.com/docs/authentication/getting-started)
````bash
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service/account/file.json
````

### Telegram 
token and settings
````bash
TELEGRAM_BOT_TOKEN=123456789:FSw4TQw4gwaRARDfasdfaW$R@qrh9jhu

# this is for debugging, set to anything to enable
TELEGRAM_BOT_DEBUG=1
````

### Style transfer
in order to get image stylized, use [this server](https://github.com/nuxdie/fast-style-transfer)
 as a reference implementation:
````bash
STYLE_SERVER_URL=https://example.com/upload
````

### Photo caption
to get a nice caption for the photo, use something like [deepai API](https://deepai.org/machine-learning-model/neuraltalk)
````bash
CAPTION_API_URL=https://api.deepai.org/api/neuraltalk
CAPTION_API_KEY=fffffff-0123-4567-8901-fffffffffffff
````

### Emoji
to have a nice emoji icons in photo caption, use something like [this serverless API](https://github.com/nuxdie/emojify)
````bash
EMOJI_API_URL=https://wt-01234567890-0.sandbox.auth0-extend.com/emojify
````

## Running
The program expects no parameters, just set environment variables correctly. It could be run like so:
````bash
$ instabot
````
After running the program sits there and listens for incoming updates from telegram.

### Docker
For convenience a simple `Dockerfile` is provided. Using it one can build a container:
_NOTE: Before building a container don't forget to place all necessary file(s) inside the folder,
such as keys, certs, JSONs._
````bash
$ docker build -t instabot .
````
And then run it like so:
````bash
$ docker run -e INSTAGRAM_USERNAME=username -e INSTAGRAM_PASSWORD=passw0rd -e TELEGRAM_BOT_TOKEN=123456789:FSw4TQw4gwaRARDfasdfaW$R@qrh9jhu -e GOOGLE_APPLICATION_CREDENTIALS=/path/to/service/account/file.json instabot
````