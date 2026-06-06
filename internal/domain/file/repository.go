package file

import "context"

type Repository interface {
	Save(ctx context.Context, file File) error
	FindByID(ctx context.Context, id string) (*File, error)
	FindByIDs(ctx context.Context, ids []string) ([]File, error)
	DeleteAll() error
}
