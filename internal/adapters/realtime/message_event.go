package realtime

import (
	"encoding/json"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

// BuildMessageEvent creates the unchanged WebSocket payload for a message.
func BuildMessageEvent(conversationID int, message conversation.Message) ([]byte, error) {
	images := make([]realtimeMessageImage, 0, len(message.Images))
	for _, image := range message.Images {
		images = append(images, realtimeMessageImage{ID: image.FileID, URL: image.URL, OriginalName: image.OriginalName})
	}
	var audio *realtimeMessageAudio
	if message.Audio != nil {
		audio = &realtimeMessageAudio{
			ID:              message.Audio.FileID,
			URL:             message.Audio.URL,
			OriginalName:    message.Audio.OriginalName,
			MimeType:        message.Audio.MimeType,
			Codec:           message.Audio.Codec,
			DurationSeconds: message.Audio.DurationSeconds,
		}
	}
	var video *realtimeMessageVideo
	if message.Video != nil {
		video = &realtimeMessageVideo{
			ID:              message.Video.FileID,
			URL:             message.Video.URL,
			OriginalName:    message.Video.OriginalName,
			MimeType:        message.Video.MimeType,
			VideoCodec:      message.Video.VideoCodec,
			AudioCodec:      message.Video.AudioCodec,
			DurationSeconds: message.Video.DurationSeconds,
			Width:           message.Video.Width,
			Height:          message.Video.Height,
		}
	}
	event := realtimeMessageEvent{
		Type:           "conversation.message.created",
		ConversationID: conversationID,
		Message: realtimeEventMessage{
			ID:         message.ID,
			SenderRole: message.SenderRole,
			Content:    message.Content,
			Images:     images,
			Audio:      audio,
			Video:      video,
			CreatedOn:  message.CreatedOn,
		},
	}
	return json.Marshal(event)
}

type realtimeMessageEvent struct {
	Type           string               `json:"type"`
	ConversationID int                  `json:"conversation_id"`
	Message        realtimeEventMessage `json:"message"`
}

type realtimeEventMessage struct {
	ID         int                    `json:"id"`
	SenderRole string                 `json:"sender_role"`
	Content    string                 `json:"content"`
	Images     []realtimeMessageImage `json:"images"`
	Audio      *realtimeMessageAudio  `json:"audio,omitempty"`
	Video      *realtimeMessageVideo  `json:"video,omitempty"`
	CreatedOn  time.Time              `json:"created_on"`
}

type realtimeMessageImage struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	OriginalName string `json:"original_name"`
}

type realtimeMessageAudio struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	Codec           string `json:"codec"`
	DurationSeconds int    `json:"duration_seconds"`
}

type realtimeMessageVideo struct {
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
