package provider_handler

import (
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"testing"
)

type conversionServiceMock struct{ mock.Mock }

func (s *conversionServiceMock) Query(ctx context.Context, id string, input provider.ConversionQueryInput) (*readmodel.Conversion, error) {
	args := s.Called(ctx, id, input)
	if result := args.Get(0); result != nil {
		return result.(*readmodel.Conversion), args.Error(1)
	}
	return nil, args.Error(1)
}
func conversionRequest(t *testing.T, service ConversionService, query string, auth bool, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/", func(c *gin.Context) {
		if auth {
			c.Set("userID", "auth0|provider")
		}
		NewConversionHandler(service).Get(c)
	})
	request := httptest.NewRequest(http.MethodGet, "/?"+query, nil).WithContext(ctx)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}
