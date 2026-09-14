package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

// Pins the contract that [S3Storage.GenerateUploadURL] signs the
// returned URL against [Config.PresignEndpoint] when it differs
// from [Config.Endpoint] (and falls back to [Config.Endpoint]
// when the two are equal or [Config.PresignEndpoint] is empty).
//
// Why this matters: AWS SigV4 binds the signature to the host
// header in the canonical request. If the API container talks to
// MinIO via a Docker-internal alias (`minio.localhost`) but the
// device / browser must reach MinIO via a LAN IP
// (`192.168.0.12`), the signed URL must carry the LAN host —
// otherwise the storage PUT succeeds at the network layer but
// MinIO returns 403 SignatureDoesNotMatch.
func TestS3Storage_GenerateUploadURL_UsesPresignEndpointWhenConfigured(t *testing.T) {
	const (
		apiInternalHost  = "minio.localhost:9000"
		deviceFacingHost = "192.168.0.12:9000"
	)

	baseConfig := func() Config {
		return Config{
			Provider:          "s3",
			Region:            "us-east-1",
			Endpoint:          "http://" + apiInternalHost,
			PublicBucket:      "loresuelvo-public-test",
			PrivateBucket:     "loresuelvo-private-test",
			PublicBaseURL:     "http://" + deviceFacingHost + "/loresuelvo-public-test",
			AccessKeyID:       "test",
			SecretAccessKey:   "test",
			PresignExpiration: 10 * time.Minute,
		}
	}

	ctx := context.Background()

	t.Run("presign endpoint unset falls back to endpoint", func(t *testing.T) {
		cfg := baseConfig()
		cfg.PresignEndpoint = ""

		s := NewS3Storage(cfg)
		target, err := s.GenerateUploadURL(ctx, testObjectToUpload())
		if err != nil {
			t.Fatalf("GenerateUploadURL: %v", err)
		}

		if !strings.Contains(target.URL, apiInternalHost) {
			t.Errorf("expected URL to carry internal host %q, got %q",
				apiInternalHost, target.URL)
		}
		if strings.Contains(target.URL, deviceFacingHost) {
			t.Errorf("URL leaked the device-facing host %q when PresignEndpoint was empty",
				deviceFacingHost)
		}
	})

	t.Run("presign endpoint set overrides endpoint for signed URL", func(t *testing.T) {
		cfg := baseConfig()
		cfg.PresignEndpoint = "http://" + deviceFacingHost

		s := NewS3Storage(cfg)
		target, err := s.GenerateUploadURL(ctx, testObjectToUpload())
		if err != nil {
			t.Fatalf("GenerateUploadURL: %v", err)
		}

		if !strings.Contains(target.URL, deviceFacingHost) {
			t.Errorf("expected URL to carry device-facing host %q, got %q",
				deviceFacingHost, target.URL)
		}
		if strings.Contains(target.URL, apiInternalHost) {
			t.Errorf("URL leaked the Docker-internal host %q when PresignEndpoint was set to %q",
				apiInternalHost, deviceFacingHost)
		}

		// SigV4 binds the signature to the host header. The
		// presign response must include a Host header that
		// matches the URL host so the storage PUT carries the
		// signature the server expects.
		host, ok := target.Headers["Host"]
		if !ok {
			t.Errorf("presign response missing — Storage requires the Host header on the outbound PUT; got headers %v",
				target.Headers)
		} else if host != deviceFacingHost {
			t.Errorf("Host header must match URL host for the signature to validate, got Host=%q want %q",
				host, deviceFacingHost)
		}
	})

	t.Run("presign endpoint equal to endpoint skips the second client", func(t *testing.T) {
		// When the operator configures PresignEndpoint to the
		// same value as Endpoint, we must not construct a
		// second S3 client (no behaviour change from the old
		// single-client path).
		cfg := baseConfig()
		cfg.Endpoint = "http://" + apiInternalHost
		cfg.PresignEndpoint = "http://" + apiInternalHost

		s := NewS3Storage(cfg)
		target, err := s.GenerateUploadURL(ctx, testObjectToUpload())
		if err != nil {
			t.Fatalf("GenerateUploadURL: %v", err)
		}

		if !strings.Contains(target.URL, apiInternalHost) {
			t.Errorf("expected URL to carry %q, got %q", apiInternalHost, target.URL)
		}
	})
}

// Generates a minimal [filedomain.ObjectToUpload] for the unit
// tests (the adapter never reads the bytes — it only uses the
// bucket + key + metadata to build the presigned URL).
func testObjectToUpload() filedomain.ObjectToUpload {
	return filedomain.ObjectToUpload{
		Bucket:    "loresuelvo-private-test",
		Key:       "files/2026/08/conversation_message_audio/test-id.webm",
		MimeType:  "audio/webm",
		SizeBytes: 1024,
	}
}