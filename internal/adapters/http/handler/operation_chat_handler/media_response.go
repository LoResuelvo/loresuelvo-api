package operation_chat_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type imageResponse struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	OriginalName string `json:"original_name"`
	Description  string `json:"description,omitempty"`
}
type audioResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	Codec           string `json:"codec"`
	DurationSeconds int    `json:"duration_seconds"`
}
type videoResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	VideoCodec      string `json:"video_codec"`
	AudioCodec      string `json:"audio_codec,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
}

func mediaFromDomain(message conversation.Message, response *messageResponse) {
	for _, image := range message.Images {
		if image.URL != "" {
			response.Images = append(response.Images, imageResponse{ID: image.FileID, URL: image.URL, OriginalName: image.OriginalName, Description: image.Description})
		}
	}
	if audio := message.Audio; audio != nil && audio.URL != "" {
		response.Audio = audioFromDomain(*audio)
	}
	if video := message.Video; video != nil && video.URL != "" {
		response.Video = &videoResponse{ID: video.FileID, URL: video.URL, OriginalName: video.OriginalName, MimeType: video.MimeType, VideoCodec: video.VideoCodec, AudioCodec: video.AudioCodec, DurationSeconds: video.DurationSeconds, Width: video.Width, Height: video.Height}
	}
}
func audioFromDomain(audio filedomain.MessageAudio) *audioResponse {
	return &audioResponse{ID: audio.FileID, URL: audio.URL, OriginalName: audio.OriginalName, MimeType: audio.MimeType, Codec: audio.Codec, DurationSeconds: audio.DurationSeconds}
}
