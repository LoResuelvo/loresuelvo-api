package admin_review_handler

import (
	"context"
	"io"
	"log/slog"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
)

type serviceMock struct{ mock.Mock }

func (m *serviceMock) List(ctx context.Context, input workorder.ReviewListInput) (*workorder.AdminReviewPage, error) {
	args := m.Called(ctx, input)
	result, _ := args.Get(0).(*workorder.AdminReviewPage)
	return result, args.Error(1)
}
func (m *serviceMock) Get(ctx context.Context, auth string, id int, page workorder.ReviewPageInput, correlation string) (*workorder.AdminReviewDetail, error) {
	args := m.Called(ctx, auth, id, page, correlation)
	result, _ := args.Get(0).(*workorder.AdminReviewDetail)
	return result, args.Error(1)
}
func (m *serviceMock) Moderate(ctx context.Context, auth string, id int, input workorder.ModerationInput, correlation string) (*workorder.ReviewModerationResult, error) {
	args := m.Called(ctx, auth, id, input, correlation)
	result, _ := args.Get(0).(*workorder.ReviewModerationResult)
	return result, args.Error(1)
}

func newHandlerTestRouter(s service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "admin") })
	handler := New(s)
	router.GET("/admin/reviews", handler.List)
	router.GET("/admin/reviews/:id", handler.Get)
	router.POST("/admin/reviews/:id/moderate", handler.Moderate)
	return router
}
