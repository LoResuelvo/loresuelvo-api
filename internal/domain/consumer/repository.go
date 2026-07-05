package consumer

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type UserRepository interface {
	Save(ctx context.Context, user user.User) (user.User, error)
	FindByEmail(email string) bool
}
