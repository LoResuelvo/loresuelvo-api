package admin

import "context"

type ProfilePhotoURLResolver interface {
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
