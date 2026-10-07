package installation_handler

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type serviceMock struct{ mock.Mock }

func (m *serviceMock) Register(ctx context.Context, authID string, r installation.Registration) (*installation.Installation, bool, error) {
	a := m.Called(ctx, authID, r)
	var i *installation.Installation
	if a.Get(0) != nil {
		i = a.Get(0).(*installation.Installation)
	}
	return i, a.Bool(1), a.Error(2)
}
func (m *serviceMock) Unregister(ctx context.Context, authID, id, secret, binding string) error {
	return m.Called(ctx, authID, id, secret, binding).Error(0)
}
func handlerEngine(t *testing.T, service Service) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	authentication, err := middleware.BaseAutheticationLayer(auth0.NewFakeValidator())
	require.NoError(t, err)
	handler := NewHandler(service)
	engine.PUT("/installations/:installation_id", authentication, handler.Register)
	engine.DELETE("/installations/:installation_id", authentication, handler.Unregister)
	return engine, auth0.NewTokenBuilder().BuildToken("authenticated-actor", nil)
}
