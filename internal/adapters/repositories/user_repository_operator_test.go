package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindOperatorIDByAuthIDReturnsInternalID(t *testing.T) {
	_, _, _, database := newCategoryUnitOfWorkTest(t)
	ctx := context.Background()
	subject := "auth0|operator-" + uuid.NewString()
	operator, err := admin.NewAdmin(subject, "operator-"+uuid.NewString()+"@example.com", "Sofía", "López", nil)
	require.NoError(t, err)
	users := repositories.NewUserRepository(database)
	saved, err := users.Save(ctx, operator)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := database.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, saved.ID())
		require.NoError(t, cleanupErr)
	})

	id, err := users.FindOperatorIDByAuthID(ctx, subject)
	require.NoError(t, err)
	assert.Equal(t, saved.ID(), id)
}

func TestFindOperatorIDByAuthIDPreservesMissingUserError(t *testing.T) {
	_, _, _, database := newCategoryUnitOfWorkTest(t)
	id, err := repositories.NewUserRepository(database).FindOperatorIDByAuthID(context.Background(), "missing-"+uuid.NewString())
	assert.Zero(t, id)
	assert.True(t, errors.Is(err, sql.ErrNoRows))
}
