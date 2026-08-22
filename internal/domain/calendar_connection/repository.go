package calendarconnection

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type UserRepository interface {
	FindByAuthID(authID string) (user.User, error)
}

type AuthorizationAttemptRepository interface {
	Save(ctx context.Context, attempt *AuthorizationAttempt) error
	FindByStateDigest(ctx context.Context, stateDigest []byte) (*AuthorizationAttempt, error)
	Consume(ctx context.Context, attempt *AuthorizationAttempt) error
}

type ConnectionRepository interface {
	SaveFromAuthorization(ctx context.Context, attemptID int, connection *Connection) error
	FindByUserID(ctx context.Context, userID int) (*Connection, error)
}

type Clock = clock.Clock
