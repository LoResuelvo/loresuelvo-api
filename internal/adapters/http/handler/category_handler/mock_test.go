package category_handler

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
)

type categoryRepositoryMock struct{ mock.Mock }

func (m *categoryRepositoryMock) Save(c category.Category) (*category.Category, error) {
	a := m.Called(c)
	result, _ := a.Get(0).(*category.Category)
	return result, a.Error(1)
}
func (m *categoryRepositoryMock) ListAll() ([]category.Category, error) {
	a := m.Called()
	result, _ := a.Get(0).([]category.Category)
	return result, a.Error(1)
}
func (m *categoryRepositoryMock) FindByID(ctx context.Context, id int) (*category.Category, error) {
	a := m.Called(ctx, id)
	result, _ := a.Get(0).(*category.Category)
	return result, a.Error(1)
}

type categoryUnitOfWorkMock struct{ mock.Mock }

func (m *categoryUnitOfWorkMock) Execute(ctx context.Context, operation func(category.TransactionalStore) error) error {
	return m.Called(ctx, operation).Error(0)
}

type categoryOperatorIDFinderMock struct{ mock.Mock }

func (m *categoryOperatorIDFinderMock) FindOperatorIDByAuthID(ctx context.Context, auth string) (int, error) {
	a := m.Called(ctx, auth)
	return a.Int(0), a.Error(1)
}

type categoryImpactReaderMock struct{ mock.Mock }

func (m *categoryImpactReaderMock) FindByCategoryID(ctx context.Context, id int, at time.Time) (*category.Impact, error) {
	a := m.Called(ctx, id, at)
	result, _ := a.Get(0).(*category.Impact)
	return result, a.Error(1)
}

type categoryClockMock struct{ mock.Mock }

func (m *categoryClockMock) Now() time.Time { return m.Called().Get(0).(time.Time) }

// handlerTestRouter exercises the real service with only the supplied persistence ports.
func handlerTestRouter(repo category.Repository, uow category.UnitOfWork, operator category.OperatorIDFinder, clock *categoryClockMock, impact category.ImpactReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "admin") })
	h := NewCategoryHandler(category.NewService(repo, uow, operator, clock, impact))
	router.PATCH("/categories/:id", h.EditCategory)
	router.GET("/categories", h.ListCategories)
	router.GET("/admin/categories/:id/impact", h.GetImpact)
	return router
}

// Keep the store port checked here when its production contract evolves.
var _ category.TransactionalStore = (*categoryStoreMock)(nil)

type categoryStoreMock struct{ mock.Mock }

func (m *categoryStoreMock) FindCategory(ctx context.Context, id int) (*category.Category, error) {
	a := m.Called(ctx, id)
	r, _ := a.Get(0).(*category.Category)
	return r, a.Error(1)
}
func (m *categoryStoreMock) FindImpact(ctx context.Context, id int, at time.Time) (*category.Impact, error) {
	a := m.Called(ctx, id, at)
	r, _ := a.Get(0).(*category.Impact)
	return r, a.Error(1)
}
func (m *categoryStoreMock) SaveCategory(ctx context.Context, c category.Category) (*category.Category, error) {
	a := m.Called(ctx, c)
	r, _ := a.Get(0).(*category.Category)
	return r, a.Error(1)
}
func (m *categoryStoreMock) SaveAuditEvent(ctx context.Context, event *audit.Event) error {
	return m.Called(ctx, event).Error(0)
}
