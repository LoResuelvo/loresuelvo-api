package provider

import "context"

type FileValidator interface {
	ValidateProviderProfilePhoto(ctx context.Context, authID, fileID string) error
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
