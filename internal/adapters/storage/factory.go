package storage

import (
	"strings"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

func NewStorageFromConfig(config Config) filedomain.Storage {
	switch strings.ToLower(strings.TrimSpace(config.Provider)) {
	case "s3", "r2":
		return NewS3Storage(config)
	default:
		return NewMemoryStorage(config.PublicBaseURL)
	}
}

type Components struct {
	Storage       filedomain.Storage
	PublicBucket  string
	PrivateBucket string
}

func NewComponentsFromEnv() Components {
	config := NewConfigFromEnv()
	return Components{
		Storage:       NewStorageFromConfig(config),
		PublicBucket:  config.PublicBucket,
		PrivateBucket: config.PrivateBucket,
	}
}
