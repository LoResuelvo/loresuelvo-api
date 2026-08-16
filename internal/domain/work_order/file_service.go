package workorder

import (
	"context"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type FileService interface {
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
	PrepareWorkOrderCompletionImages(ctx context.Context, authID string, fileIDs []string) ([]filedomain.Image, error)
}
