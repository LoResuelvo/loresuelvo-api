package operation

import (
	"context"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// ChatMediaResolver admits only confirmed private message media under file policy.
type ChatMediaResolver interface {
	ResolveMessageImages(context.Context, []string) (map[string]filedomain.MessageImage, error)
	ResolveMessageAudios(context.Context, []string) (map[string]filedomain.MessageAudio, error)
	ResolveMessageVideos(context.Context, []string) (map[string]filedomain.MessageVideo, error)
}

// prepareMedia is invoked only after successful persistence of the access audit.
func (service *ChatService) prepareMedia(ctx context.Context, association *readmodel.ConversationAssociation, messages []conversation.Message) error {
	if len(messages) == 0 {
		return nil
	}
	ids := make([]int, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	refs, err := service.attachments.FindByMessagePage(ctx, conversation.MessageAttachmentScope{ConversationID: association.ConversationID, ConsumerID: association.ConsumerID, ProviderID: association.ProviderID, MessageIDs: ids})
	if err != nil {
		return fmt.Errorf("reading operation conversation attachments: %w", err)
	}
	var imageIDs, audioIDs, videoIDs []string
	// Iterate delivered IDs, not arbitrary map entries: a reader cannot broaden signing.
	for _, message := range messages {
		ref := refs[message.ID]
		for _, image := range ref.Images {
			imageIDs = append(imageIDs, image.FileID)
		}
		if ref.AudioFileID != "" {
			audioIDs = append(audioIDs, ref.AudioFileID)
		}
		if ref.VideoFileID != "" {
			videoIDs = append(videoIDs, ref.VideoFileID)
		}
	}
	images := map[string]filedomain.MessageImage{}
	audios := map[string]filedomain.MessageAudio{}
	videos := map[string]filedomain.MessageVideo{}
	if len(imageIDs) > 0 {
		images, err = service.media.ResolveMessageImages(ctx, imageIDs)
		if err != nil {
			return fmt.Errorf("resolving operation conversation images: %w", err)
		}
	}
	if len(audioIDs) > 0 {
		audios, err = service.media.ResolveMessageAudios(ctx, audioIDs)
		if err != nil {
			return fmt.Errorf("resolving operation conversation audios: %w", err)
		}
	}
	if len(videoIDs) > 0 {
		videos, err = service.media.ResolveMessageVideos(ctx, videoIDs)
		if err != nil {
			return fmt.Errorf("resolving operation conversation videos: %w", err)
		}
	}
	for index := range messages {
		message := &messages[index]
		ref := refs[message.ID]
		// Do not reuse legacy media fields from the message text reader.
		message.Images = nil
		message.Audio = nil
		message.Video = nil
		for _, reference := range ref.Images {
			if image, ok := images[reference.FileID]; ok {
				image.Description = reference.Description
				message.Images = append(message.Images, image)
			}
		}
		if audio, ok := audios[ref.AudioFileID]; ok {
			message.Audio = &audio
		}
		if video, ok := videos[ref.VideoFileID]; ok {
			message.Video = &video
		}
	}
	return nil
}
