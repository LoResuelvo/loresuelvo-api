package operation_media_handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/operation_media_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mediaHandlerServiceStub struct {
	image *operation.OperationImage
	err   error
}

func (stub mediaHandlerServiceStub) Get(context.Context, string, string) (*operation.OperationImage, error) {
	return stub.image, stub.err
}

func mediaResponse(t *testing.T, service mediaHandlerServiceStub) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/operations/:operation_id/images/:file_id", operation_media_handler.NewHandler(service).Get)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/operations/jr-1/images/file", nil))
	return response
}

func TestMediaHandlerReturnsPrivateImageBytes(t *testing.T) {
	response := mediaResponse(t, mediaHandlerServiceStub{image: &operation.OperationImage{Bytes: []byte("abc"), MimeType: "image/png"}})
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "abc", response.Body.String())
	require.Equal(t, "image/png", response.Header().Get("Content-Type"))
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
}

func TestMediaHandlerMapsFailuresWithoutLeakingBytes(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{"invalid operation", operation.ErrInvalidOperationID, http.StatusBadRequest},
		{"invalid image", operation.ErrInvalidOperationImageID, http.StatusBadRequest},
		{"not found", operation.ErrOperationImageNotFound, http.StatusNotFound},
		{"storage failure", errors.New("secret storage error"), http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := mediaResponse(t, mediaHandlerServiceStub{err: test.err})
			require.Equal(t, test.status, response.Code)
			require.NotContains(t, response.Body.String(), "secret storage error")
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		})
	}
}
