package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type MemoryStorage struct {
	publicBaseURL string
	objects       map[string]filedomain.ObjectMetadata
	mu            sync.RWMutex
}

func NewMemoryStorage(publicBaseURL string) *MemoryStorage {
	if strings.TrimSpace(publicBaseURL) == "" {
		publicBaseURL = "http://storage.local"
	}
	return &MemoryStorage{
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		objects:       map[string]filedomain.ObjectMetadata{},
	}
}

func (storage *MemoryStorage) GenerateUploadURL(_ context.Context, object filedomain.ObjectToUpload) (*filedomain.UploadTarget, error) {
	storage.mu.Lock()
	storage.objects[objectIdentity(object.Bucket, object.Key)] = filedomain.ObjectMetadata{
		MimeType:  object.MimeType,
		SizeBytes: object.SizeBytes,
	}
	storage.mu.Unlock()

	return &filedomain.UploadTarget{
		URL: fmt.Sprintf("%s/upload/%s/%s", storage.publicBaseURL, object.Bucket, object.Key),
		Headers: map[string]string{
			"Content-Type": object.MimeType,
		},
	}, nil
}

func (storage *MemoryStorage) ReadObjectMetadata(_ context.Context, bucket, key string) (*filedomain.ObjectMetadata, error) {
	storage.mu.RLock()
	metadata, ok := storage.objects[objectIdentity(bucket, key)]
	storage.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("object metadata not found")
	}
	return &metadata, nil
}

func (storage *MemoryStorage) PublicURL(bucket, key string) string {
	return fmt.Sprintf("%s/%s/%s", storage.publicBaseURL, bucket, key)
}

func objectIdentity(bucket, key string) string {
	return bucket + "/" + key
}
