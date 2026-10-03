package bootstrap

import (
	"database/sql"
	"testing"

	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	mercadopago "github.com/LoResuelvo/loresuelvo-api/internal/adapters/payment_account/mercadopago"
	"github.com/stretchr/testify/require"
)

func TestDependenciesWireProviderReputationReaderAndHandler(t *testing.T) {
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("STORAGE_PROVIDER", "memory")
	dependencies, err := newDependencies(&sql.DB{}, dependencyAdapters{
		paymentAccountOAuthConnector: mercadopago.NewFakeOAuthClient(),
		auditCursorSigningKey:        []byte("test-bootstrap-reputation-cursor-key-private"),
	})
	require.NoError(t, err)
	require.NotNil(t, dependencies.Persistence.ProviderReputationReader)
	require.NotNil(t, dependencies.RouterConfig(nil, nil, httpadapter.TestEnvironment).ReputationHandler)
}

func TestDependenciesRejectShortSharedPrivateCursorKey(t *testing.T) {
	t.Setenv("ENVIRONMENT", "test")
	t.Setenv("STORAGE_PROVIDER", "memory")
	dependencies, err := newDependencies(&sql.DB{}, dependencyAdapters{auditCursorSigningKey: []byte("short")})
	require.Nil(t, dependencies)
	require.ErrorContains(t, err, "cursor signing key must be at least 32 bytes")
}
