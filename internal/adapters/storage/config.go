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
	// PresignEndpoint is the host the presigned S3 URLs are signed
	// against. When empty, the presign client uses [Endpoint]
	// (which works when the API container and the storage live on
	// the same network — Docker Compose, single-host deploy).
	//
	// Set this to a host the *client* (browser, Android device,
	// webapp) can reach when the API's storage host is a
	// Docker-internal alias the client can't resolve (e.g. the
	// default `minio.localhost` alias). The signature is bound
	// to the host header, so the presign endpoint and the URL
	// the client PUTs to must agree; a separate presign client
	// lets the API keep using the internal alias for its own
	// `GetObject` / `HeadObject` calls.
	PresignEndpoint   string
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
		PresignEndpoint:   strings.TrimSpace(os.Getenv("STORAGE_PRESIGN_ENDPOINT")),
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
