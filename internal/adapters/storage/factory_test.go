package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewStorageFromConfigRejectsUnknownProvider(t *testing.T) {
	_, err := NewStorageFromConfig(Config{Provider: "filesystem"})

	require.ErrorIs(t, err, ErrUnsupportedProvider)
}

func TestNewStorageFromConfigRejectsMemoryInStagingAndProduction(t *testing.T) {
	for _, environment := range []string{"staging", "production"} {
		t.Run(environment, func(t *testing.T) {
			_, err := NewStorageFromConfig(Config{
				Environment: environment,
				Provider:    "memory",
			})

			require.ErrorIs(t, err, ErrMemoryStorageNotAllowed)
		})
	}
}

func TestNewStorageFromConfigAllowsMemoryOutsideStagingAndProduction(t *testing.T) {
	for _, environment := range []string{"development", "dev", "test"} {
		t.Run(environment, func(t *testing.T) {
			created, err := NewStorageFromConfig(Config{
				Environment: environment,
				Provider:    "memory",
			})

			require.NoError(t, err)
			require.IsType(t, &MemoryStorage{}, created)
		})
	}
}

func TestNewStorageFromConfigRequiresCloudCredentialsRegionAndBuckets(t *testing.T) {
	base := validCloudConfig()
	cases := map[string]func(*Config){
		"access key":     func(config *Config) { config.AccessKeyID = "" },
		"secret key":     func(config *Config) { config.SecretAccessKey = "" },
		"region":         func(config *Config) { config.Region = "" },
		"public bucket":  func(config *Config) { config.PublicBucket = "" },
		"private bucket": func(config *Config) { config.PrivateBucket = "" },
	}

	for _, provider := range []string{"s3", "r2"} {
		for name, mutate := range cases {
			t.Run(provider+"/"+name, func(t *testing.T) {
				config := base
				config.Provider = provider
				mutate(&config)

				_, err := NewStorageFromConfig(config)

				require.ErrorIs(t, err, ErrIncompleteCloudConfig)
			})
		}
	}
}

func TestNewStorageFromConfigCreatesS3CompatibleStorage(t *testing.T) {
	for _, provider := range []string{"s3", "r2"} {
		t.Run(provider, func(t *testing.T) {
			config := validCloudConfig()
			config.Provider = provider

			created, err := NewStorageFromConfig(config)

			require.NoError(t, err)
			require.IsType(t, &S3Storage{}, created)
		})
	}
}

func validCloudConfig() Config {
	return Config{
		Environment:       "production",
		Provider:          "s3",
		Endpoint:          "https://storage.example.com",
		Region:            "us-east-1",
		PublicBucket:      "public",
		PrivateBucket:     "private",
		AccessKeyID:       "access-key",
		SecretAccessKey:   "secret-key",
		PresignExpiration: time.Minute,
	}
}
