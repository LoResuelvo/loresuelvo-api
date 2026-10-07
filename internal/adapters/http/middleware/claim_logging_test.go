package middleware

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaimAndFileLoggingKeepsOnlyTechnicalMetadata(t *testing.T) {
	for _, path := range []string{"/claims", "/claims/7", "/files/presign", "/files/private-file-sentinel/confirm"} {
		for _, status := range []int{200, 400, 401, 404, 500} {
			t.Run(path+strconv.Itoa(status), func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				var output bytes.Buffer
				logger := slog.New(slog.NewJSONHandler(&output, nil))
				engine := gin.New()
				engine.Use(RequestLogger(logger))
				engine.POST("/claims", func(c *gin.Context) {
					c.JSON(status, gin.H{"description": "testimony-secret", "resolution": "reasoning-secret", "url": "https://private/signed-secret", "image_file_ids": []string{"private-file-sentinel"}})
				})
				engine.POST("/claims/:id", func(c *gin.Context) {
					c.JSON(status, gin.H{"resolution": "reasoning-secret", "images": []gin.H{{"file_id": "private-file-sentinel", "url": "https://private/signed-secret"}}})
				})
				engine.POST("/files/presign", func(c *gin.Context) {
					c.JSON(status, gin.H{"key": "bucket-key-secret", "url": "signed-secret", "file_id": "private-file-sentinel"})
				})
				engine.POST("/files/:file_id/confirm", func(c *gin.Context) {
					c.JSON(status, gin.H{"key": "bucket-key-secret", "file_id": "private-file-sentinel"})
				})
				request := httptest.NewRequest("POST", path+"?category_id=query-secret", strings.NewReader(`{"reason":"reason-secret","description":"testimony-secret","original_name":"original-name-secret","purpose":"claim_evidence_image","key":"bucket-key-secret"}`))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				require.Equal(t, status, response.Code)
				for _, secret := range []string{"testimony-secret", "reason-secret", "reasoning-secret", "signed-secret", "private-file-sentinel", "query-secret", "bucket-key-secret", "original-name-secret", "claim_evidence_image", "http.request_body", "http.response_body", "http.path_params"} {
					require.NotContains(t, output.String(), secret)
				}
				require.Contains(t, output.String(), "http.status_code")
				require.Contains(t, output.String(), "http.route")
			})
		}
	}
}
