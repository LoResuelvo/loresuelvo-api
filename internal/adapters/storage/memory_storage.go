package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type MemoryStorage struct {
	publicBaseURL  string
	objects        map[string]memoryObject
	mu             sync.RWMutex
	uploadPolicies map[string]filedomain.ObjectToUpload
}

type memoryObject struct {
	metadata filedomain.ObjectMetadata
	data     []byte
}

func NewMemoryStorage(publicBaseURL string) *MemoryStorage {
	if strings.TrimSpace(publicBaseURL) == "" {
		publicBaseURL = "http://storage.local"
	}
	return &MemoryStorage{
		publicBaseURL:  strings.TrimRight(publicBaseURL, "/"),
		objects:        map[string]memoryObject{},
		uploadPolicies: map[string]filedomain.ObjectToUpload{},
	}
}

func (storage *MemoryStorage) GenerateUploadURL(_ context.Context, object filedomain.ObjectToUpload) (*filedomain.UploadTarget, error) {
	storage.mu.Lock()
	storage.uploadPolicies[objectIdentity(object.Bucket, object.Key)] = object
	if !object.CreateOnly {
		storage.objects[objectIdentity(object.Bucket, object.Key)] = memoryObject{
			metadata: filedomain.ObjectMetadata{
				MimeType:  object.MimeType,
				SizeBytes: object.SizeBytes,
			},
			data: make([]byte, object.SizeBytes),
		}
	}
	storage.mu.Unlock()

	headers := map[string]string{"Content-Type": object.MimeType}
	if object.CreateOnly {
		headers["If-None-Match"] = "*"
	}
	return &filedomain.UploadTarget{
		URL:     fmt.Sprintf("%s/upload/%s/%s", storage.publicBaseURL, object.Bucket, object.Key),
		Headers: headers,
	}, nil
}

func (storage *MemoryStorage) ReadObjectMetadata(_ context.Context, bucket, key string) (*filedomain.ObjectMetadata, error) {
	storage.mu.RLock()
	object, ok := storage.objects[objectIdentity(bucket, key)]
	storage.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("object metadata not found")
	}
	metadata := object.metadata
	return &metadata, nil
}

func (storage *MemoryStorage) ReadObject(_ context.Context, object filedomain.ObjectToDownload) ([]byte, error) {
	storage.mu.RLock()
	storedObject, ok := storage.objects[objectIdentity(object.Bucket, object.Key)]
	storage.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	if object.MaxSizeBytes > 0 && len(storedObject.data) > object.MaxSizeBytes {
		return nil, fmt.Errorf("object exceeds maximum size")
	}
	data := make([]byte, len(storedObject.data))
	copy(data, storedObject.data)
	return data, nil
}

func (storage *MemoryStorage) GenerateDownloadURL(_ context.Context, object filedomain.ObjectToDownload) (string, error) {
	storage.mu.RLock()
	_, ok := storage.objects[objectIdentity(object.Bucket, object.Key)]
	storage.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("object not found")
	}
	return fmt.Sprintf("%s/download/%s/%s", storage.publicBaseURL, object.Bucket, object.Key), nil
}

func (storage *MemoryStorage) PublicURL(bucket, key string) string {
	return fmt.Sprintf("%s/%s/%s", storage.publicBaseURL, bucket, key)
}

func objectIdentity(bucket, key string) string {
	return bucket + "/" + key
}

// PutObject simulates a signed upload, enforcing its required headers atomically.
func (storage *MemoryStorage) PutObject(ctx context.Context, bucket, key string, headers map[string]string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	storage.mu.Lock()
	defer storage.mu.Unlock()
	identity := objectIdentity(bucket, key)
	policy, ok := storage.uploadPolicies[identity]
	if !ok {
		return fmt.Errorf("upload target not found")
	}
	if headers["Content-Type"] != policy.MimeType || len(data) != policy.SizeBytes {
		return fmt.Errorf("upload metadata mismatch")
	}
	if policy.CreateOnly {
		if headers["If-None-Match"] != "*" {
			return fmt.Errorf("required upload header missing")
		}
		if _, exists := storage.objects[identity]; exists {
			return fmt.Errorf("object already exists")
		}
	}
	storage.objects[identity] = memoryObject{metadata: filedomain.ObjectMetadata{MimeType: policy.MimeType, SizeBytes: len(data)}, data: append([]byte(nil), data...)}
	return nil
}
