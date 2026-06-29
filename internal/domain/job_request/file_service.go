package jobrequest

import (
	"context"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type FileService interface {
	PrepareJobRequestImages(ctx context.Context, authID string, fileIDs []string) ([]filedomain.Image, error)
	ResolveJobRequestImages(ctx context.Context, images []filedomain.Image) ([]filedomain.Image, error)
}
