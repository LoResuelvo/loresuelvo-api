package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestClaimEvidenceMemoryUploadIsCreateOnly(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage("")
	target, err := storage.GenerateUploadURL(ctx, filedomain.ObjectToUpload{Bucket: "private", Key: "evidence", MimeType: "image/png", SizeBytes: 3, CreateOnly: true})
	require.NoError(t, err)
	require.Equal(t, "*", target.Headers["If-None-Match"])
	_, err = storage.ReadObjectMetadata(ctx, "private", "evidence")
	require.Error(t, err)
	require.Error(t, storage.PutObject(ctx, "private", "evidence", map[string]string{"Content-Type": "image/png"}, []byte("one")))
	require.NoError(t, storage.PutObject(ctx, "private", "evidence", target.Headers, []byte("one")))
	require.Error(t, storage.PutObject(ctx, "private", "evidence", target.Headers, []byte("two")))
	_, err = storage.GenerateUploadURL(ctx, filedomain.ObjectToUpload{Bucket: "private", Key: "evidence", MimeType: "image/png", SizeBytes: 3, CreateOnly: true})
	require.NoError(t, err)
	data, err := storage.ReadObject(ctx, filedomain.ObjectToDownload{Bucket: "private", Key: "evidence"})
	require.NoError(t, err)
	require.Equal(t, []byte("one"), data)
}

func TestClaimEvidenceS3UploadSignsCreateOnlyHeader(t *testing.T) {
	storage := NewS3Storage(validCloudConfig())
	target, err := storage.GenerateUploadURL(context.Background(), filedomain.ObjectToUpload{Bucket: "private", Key: "evidence", MimeType: "image/png", SizeBytes: 3, CreateOnly: true})
	require.NoError(t, err)
	parsed, err := url.Parse(target.URL)
	require.NoError(t, err)
	require.Contains(t, strings.Split(parsed.Query().Get("X-Amz-SignedHeaders"), ";"), "if-none-match")
	require.Equal(t, "*", target.Headers["If-None-Match"])
}

// TestClaimEvidenceMinIOReplay runs only against the explicitly configured local test storage.
func TestClaimEvidenceMinIOReplay(t *testing.T) {
	config := NewConfigFromEnv()
	testBucket := os.Getenv("TEST_STORAGE_PRIVATE_BUCKET")
	if config.Provider != "s3" || config.Endpoint == "" || testBucket == "" {
		t.Skip("test S3 storage not configured")
	}
	require.Equal(t, testBucket, config.PrivateBucket, "test storage bucket must be selected by the test runner")
	require.True(t, strings.HasSuffix(testBucket, "-test"), "refusing non-test storage bucket")
	endpoint, err := url.Parse(config.Endpoint)
	require.NoError(t, err)
	require.Contains(t, []string{"minio.localhost", "minio", "localhost", "127.0.0.1", "::1"}, endpoint.Hostname(), "refusing non-local storage endpoint")
	require.NotContains(t, []string{"staging", "production"}, config.Environment, "refusing remote runtime environment")
	storage := NewS3Storage(config)
	ctx := context.Background()
	object := filedomain.ObjectToUpload{Bucket: config.PrivateBucket, Key: "claim-evidence-test/" + uuid.NewString(), MimeType: "image/png", SizeBytes: 3, CreateOnly: true}
	target, err := storage.GenerateUploadURL(ctx, object)
	require.NoError(t, err)
	put := func(data string, includeHeader bool) int {
		request, err := http.NewRequestWithContext(ctx, http.MethodPut, target.URL, bytes.NewBufferString(data))
		require.NoError(t, err)
		for key, value := range target.Headers {
			if includeHeader || !strings.EqualFold(key, "If-None-Match") {
				request.Header.Set(key, value)
			}
		}
		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		return response.StatusCode
	}
	require.Equal(t, http.StatusOK, put("one", true))
	require.Equal(t, http.StatusPreconditionFailed, put("two", true))
	require.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, put("two", false))
	data, err := storage.ReadObject(ctx, filedomain.ObjectToDownload{Bucket: object.Bucket, Key: object.Key})
	require.NoError(t, err)
	require.Equal(t, []byte("one"), data)
}
