package user

import "context"

type ProfilePhotoURLResolver interface {
	ResolvePublicURL(ctx context.Context, fileID string) (string, error)
}
