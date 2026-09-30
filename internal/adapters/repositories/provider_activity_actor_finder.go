package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

// ProviderActivityActorFinder resolves the authenticated identity without
// hydrating a user profile or loading unrelated provider data.
type ProviderActivityActorFinder struct{ db *sql.DB }

func NewProviderActivityActorFinder(db *sql.DB) *ProviderActivityActorFinder {
	return &ProviderActivityActorFinder{db: db}
}

var _ provider.ProviderActorFinder = (*ProviderActivityActorFinder)(nil)

func (f *ProviderActivityActorFinder) FindByAuthID(ctx context.Context, authID string) (int, string, error) {
	var providerID int
	var role string
	err := f.db.QueryRowContext(ctx, `
		SELECT CASE WHEN u.role = 'provider' THEN COALESCE(p.user_id, 0) ELSE 0 END, u.role
		FROM users u
		LEFT JOIN providers p ON p.user_id = u.id
		WHERE u.auth_id = $1`, authID).Scan(&providerID, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", user.ErrNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("finding provider activity actor: %w", err)
	}
	return providerID, role, nil
}
