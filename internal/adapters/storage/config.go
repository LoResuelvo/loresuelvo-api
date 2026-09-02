package storage

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultPresignExpiration = 10 * time.Minute

type Config struct {
	Environment       string
	Provider          string
	Endpoint          string
	Region            string
	PublicBucket      string
	PrivateBucket     string
	PublicBaseURL     string
	AccessKeyID       string
	SecretAccessKey   string
	PresignExpiration time.Duration
}

func NewConfigFromEnv() Config {
	return Config{
		Environment:       envOrDefault("ENVIRONMENT", "development"),
		Provider:          envOrDefault("STORAGE_PROVIDER", "memory"),
		Endpoint:          strings.TrimSpace(os.Getenv("STORAGE_ENDPOINT")),
		Region:            envOrDefault("STORAGE_REGION", "auto"),
		PublicBucket:      envOrDefault("STORAGE_PUBLIC_BUCKET", "loresuelvo-public-local"),
		PrivateBucket:     envOrDefault("STORAGE_PRIVATE_BUCKET", "loresuelvo-private-local"),
		PublicBaseURL:     strings.TrimSpace(os.Getenv("STORAGE_PUBLIC_BASE_URL")),
		AccessKeyID:       strings.TrimSpace(os.Getenv("STORAGE_ACCESS_KEY_ID")),
		SecretAccessKey:   strings.TrimSpace(os.Getenv("STORAGE_SECRET_ACCESS_KEY")),
		PresignExpiration: presignExpirationFromEnv(),
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func presignExpirationFromEnv() time.Duration {
	value := strings.TrimSpace(os.Getenv("STORAGE_PRESIGN_EXPIRATION_SECONDS"))
	if value == "" {
		return defaultPresignExpiration
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return defaultPresignExpiration
	}
	return time.Duration(seconds) * time.Second
}
