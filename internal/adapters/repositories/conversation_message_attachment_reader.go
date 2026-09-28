package repositories

import (
	"context"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

type ConversationMessageAttachmentReader struct {
	images *MessageImageRepository
	audios *MessageAudioRepository
	videos *MessageVideoRepository
}

func NewConversationMessageAttachmentReader(images *MessageImageRepository, audios *MessageAudioRepository, videos *MessageVideoRepository) *ConversationMessageAttachmentReader {
	return &ConversationMessageAttachmentReader{images: images, audios: audios, videos: videos}
}
func (reader *ConversationMessageAttachmentReader) FindByMessagePage(ctx context.Context, scope conversation.MessageAttachmentScope) (map[int]conversation.MessageAttachmentReferences, error) {
	if err := validateMessageAttachmentScope(scope); err != nil {
		return nil, err
	}
	result := make(map[int]conversation.MessageAttachmentReferences, len(scope.MessageIDs))
	if len(scope.MessageIDs) == 0 {
		return result, nil
	}
	images, err := reader.images.FindByMessagePage(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("reading page images: %w", err)
	}
	audios, err := reader.audios.FindByMessagePage(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("reading page audios: %w", err)
	}
	videos, err := reader.videos.FindByMessagePage(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("reading page videos: %w", err)
	}
	for id, refs := range images {
		entry := result[id]
		entry.Images = refs
		result[id] = entry
	}
	for id, fileID := range audios {
		entry := result[id]
		entry.AudioFileID = fileID
		result[id] = entry
	}
	for id, fileID := range videos {
		entry := result[id]
		entry.VideoFileID = fileID
		result[id] = entry
	}
	return result, nil
}
