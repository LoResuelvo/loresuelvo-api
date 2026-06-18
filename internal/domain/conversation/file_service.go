package conversation

import "context"

type FileURLResolver interface {
	ResolvePublicURL(ctx context.Context, fileID string) (string, error)
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
