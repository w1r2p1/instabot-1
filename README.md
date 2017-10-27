# InstaBot
## Project goal
The aim of this project is to build a telegram bot, that accepts an image as a file or photo and uploads it to
Instagram. It also adds appropriate hashtags and geotags based on image data. Video and gallery support could be added at
later stages of the project.

## Setup
In order to get things working user needs to provide these environment variables:

- Instagram credentials for an account to post to
````bash
INSTAGRAM_USERNAME=username
INSTAGRAM_PASSWORD=passw0rd
````

- Google Vision API credentials for adding appropriate #hashtags,
see [this guide to setup](https://cloud.google.com/docs/authentication/getting-started)
````bash
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service/account/file.json
````

## Running
The program expects a single parameter: photo file url. It could be run like so:
````bash
$ instabot https://bellard.org/bpg/lena30.jpg
````