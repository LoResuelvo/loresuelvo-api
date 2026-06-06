package file

import "context"

type Storage interface {
	GenerateUploadURL(ctx context.Context, object ObjectToUpload) (*UploadTarget, error)
	ReadObjectMetadata(ctx context.Context, bucket, key string) (*ObjectMetadata, error)
	PublicURL(bucket, key string) string
}

type ObjectToUpload struct {
	Bucket    string
	Key       string
	MimeType  string
	SizeBytes int
}

type UploadTarget struct {
	URL     string
	Headers map[string]string
}

type ObjectMetadata struct {
	MimeType  string
	SizeBytes int
}
