package conversation

import (
	"context"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type FileService interface {
	ResolvePublicURL(ctx context.Context, fileID string) (string, error)
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
	PrepareMessageImages(ctx context.Context, authID string, fileIDs []string) ([]filedomain.MessageImage, error)
	PrepareMessageAudio(ctx context.Context, authID, fileID string) (*filedomain.MessageAudio, error)
	PrepareChatbotMessageImages(ctx context.Context, authID string, fileIDs []string) ([]filedomain.MessageImageContent, error)
	ResolveMessageImages(ctx context.Context, fileIDs []string) (map[string]filedomain.MessageImage, error)
	ResolveMessageAudios(ctx context.Context, fileIDs []string) (map[string]filedomain.MessageAudio, error)
}
