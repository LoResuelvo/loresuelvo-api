package conversation

import (
	"strings"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

const (
	SenderConsumer = "consumer"
	SenderProvider = "provider"
	SenderChatbot  = "chatbot"
)

type Message struct {
	ID             int
	ConversationID int
	SenderRole     string
	Content        string
	Images         []filedomain.MessageImage
	Audio          *filedomain.MessageAudio
	Video          *filedomain.MessageVideo
	CreatedOn      time.Time
}

func NewConsumerMessage(content string, images ...filedomain.MessageImage) (*Message, error) {
	return newMessage(SenderConsumer, content, images)
}

func NewProviderMessage(content string, images ...filedomain.MessageImage) (*Message, error) {
	return newMessage(SenderProvider, content, images)
}

func NewConsumerAudioMessage(audio filedomain.MessageAudio) (*Message, error) {
	return newAudioMessage(SenderConsumer, audio)
}

func NewProviderAudioMessage(audio filedomain.MessageAudio) (*Message, error) {
	return newAudioMessage(SenderProvider, audio)
}

func NewConsumerVideoMessage(content string, video filedomain.MessageVideo) (*Message, error) {
	return newVideoMessage(SenderConsumer, content, video)
}

func NewProviderVideoMessage(content string, video filedomain.MessageVideo) (*Message, error) {
	return newVideoMessage(SenderProvider, content, video)
}

func NewChatbotMessage(content string) (*Message, error) {
	return newMessage(SenderChatbot, content, nil)
}

func newMessage(senderRole, content string, images []filedomain.MessageImage) (*Message, error) {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" && len(images) == 0 {
		return nil, ErrMessageRequired
	}
	messageImages, err := ensureMessageImages(images)
	if err != nil {
		return nil, err
	}

	return &Message{
		SenderRole: senderRole,
		Content:    trimmedContent,
		Images:     messageImages,
	}, nil
}

func newAudioMessage(senderRole string, audio filedomain.MessageAudio) (*Message, error) {
	if strings.TrimSpace(audio.FileID) == "" {
		return nil, ErrMessageAudioNotAvailable
	}
	if strings.TrimSpace(audio.OriginalName) == "" || audio.DurationSeconds <= 0 || strings.TrimSpace(audio.Codec) == "" {
		return nil, ErrMessageAudioNotAvailable
	}

	audio.FileID = strings.TrimSpace(audio.FileID)
	audio.OriginalName = strings.TrimSpace(audio.OriginalName)
	audio.Codec = strings.ToLower(strings.TrimSpace(audio.Codec))
	return &Message{SenderRole: senderRole, Audio: &audio}, nil
}

func newVideoMessage(senderRole, content string, video filedomain.MessageVideo) (*Message, error) {
	trimmedContent := strings.TrimSpace(content)
	if strings.TrimSpace(video.FileID) == "" || strings.TrimSpace(video.OriginalName) == "" {
		return nil, ErrMessageVideoNotAvailable
	}
	if video.DurationSeconds <= 0 || strings.TrimSpace(video.VideoCodec) == "" || video.Width <= 0 || video.Height <= 0 {
		return nil, ErrMessageVideoNotAvailable
	}

	video.FileID = strings.TrimSpace(video.FileID)
	video.OriginalName = strings.TrimSpace(video.OriginalName)
	video.MimeType = strings.ToLower(strings.TrimSpace(video.MimeType))
	video.VideoCodec = strings.ToLower(strings.TrimSpace(video.VideoCodec))
	video.AudioCodec = strings.ToLower(strings.TrimSpace(video.AudioCodec))
	return &Message{SenderRole: senderRole, Content: trimmedContent, Video: &video}, nil
}

func ensureMessageImages(images []filedomain.MessageImage) ([]filedomain.MessageImage, error) {
	seen := make(map[string]struct{}, len(images))
	messageImages := make([]filedomain.MessageImage, len(images))
	for index, image := range images {
		image.FileID = strings.TrimSpace(image.FileID)
		if image.FileID == "" {
			return nil, ErrMessageImageNotAvailable
		}
		if _, exists := seen[image.FileID]; exists {
			return nil, ErrMessageImageNotAvailable
		}
		seen[image.FileID] = struct{}{}
		messageImages[index] = image
	}
	return messageImages, nil
}
