// this package exports common struct
package metadata

type PhotoMetadata struct {
	PhotoId      string `json:"photo_id"      mapstructure:"photo_id"`
	ChatId       int64  `json:"chat_id"       mapstructure:"chat_id"`
	PhotoUrl     string `json:"photo_url"     mapstructure:"photo_url"`
	Caption      string `json:"caption"       mapstructure:"caption"`
	FinalCaption string `json:"final_caption" mapstructure:"final_caption"`
	CaptionRu    string `json:"caption_ru"    mapstructure:"caption_ru"`
	Hashtag      string `json:"hashtag"       mapstructure:"hashtag"`
	HashtagRu    string `json:"hashtag_ru"    mapstructure:"hashtag_ru"`
	StyledUrl    string `json:"styled_url"    mapstructure:"styled_url"`
	Publish      bool   `json:"publish"       mapstructure:"publish"`
	Published    bool   `json:"published"     mapstructure:"published"`
	PublishedUrl string `json:"published_url" mapstructure:"published_url"`
	NSFW         bool   `json:"nsfw"          mapstructure:"nsfw"`
	NSFWChecked  bool   `json:"nsfw_checked"  mapstructure:"nsfw_checked"`
}

type ChannelMessage struct {
	Type				 string `json:"type"      	  mapstructure:"type"`
	PhotoId      string `json:"photo_id"      mapstructure:"photo_id"`
	Message      string `json:"message"       mapstructure:"message"`
}