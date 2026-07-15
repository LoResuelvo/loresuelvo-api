package provider

import "context"

type FileService interface {
	ValidateProfilePhoto(ctx context.Context, authID, fileID string) error
	ResolvePublicURL(ctx context.Context, fileID string) (string, error)
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
