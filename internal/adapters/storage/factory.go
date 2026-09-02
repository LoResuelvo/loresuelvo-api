package storage

import (
	"errors"
	"fmt"
	"os"
	"strings"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

var (
	ErrUnsupportedProvider     = errors.New("unsupported storage provider")
	ErrMemoryStorageNotAllowed = errors.New("memory storage is not allowed in this environment")
	ErrIncompleteCloudConfig   = errors.New("cloud storage configuration is incomplete")
)

func NewStorageFromConfig(config Config) (filedomain.Storage, error) {
	provider := strings.ToLower(strings.TrimSpace(config.Provider))
	if provider == "" {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProvider, config.Provider)
	}

	environment := strings.ToLower(strings.TrimSpace(config.Environment))
	if environment == "" {
		environment = strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	}
	if environment == "" {
		environment = "development"
	}

	switch provider {
	case "memory":
		if environment == "staging" || environment == "production" {
			return nil, fmt.Errorf("%w: %s", ErrMemoryStorageNotAllowed, environment)
		}
		return NewMemoryStorage(config.PublicBaseURL), nil
	case "s3", "r2":
		if err := validateCloudConfig(config); err != nil {
			return nil, fmt.Errorf("%w for %s: %v", ErrIncompleteCloudConfig, provider, err)
		}
		return NewS3Storage(config), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, config.Provider)
	}
}

func validateCloudConfig(config Config) error {
	missing := make([]string, 0, 5)
	if strings.TrimSpace(config.AccessKeyID) == "" {
		missing = append(missing, "access key id")
	}
	if strings.TrimSpace(config.SecretAccessKey) == "" {
		missing = append(missing, "secret access key")
	}
	if strings.TrimSpace(config.Region) == "" {
		missing = append(missing, "region")
	}
	if strings.TrimSpace(config.PublicBucket) == "" {
		missing = append(missing, "public bucket")
	}
	if strings.TrimSpace(config.PrivateBucket) == "" {
		missing = append(missing, "private bucket")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}
	return nil
}

type Components struct {
	Storage       filedomain.Storage
	PublicBucket  string
	PrivateBucket string
}

func NewComponentsFromEnv() (Components, error) {
	config := NewConfigFromEnv()
	storage, err := NewStorageFromConfig(config)
	if err != nil {
		return Components{}, err
	}
	return Components{
		Storage:       storage,
		PublicBucket:  config.PublicBucket,
		PrivateBucket: config.PrivateBucket,
	}, nil
}
