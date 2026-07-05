package user

import "context"

type Repository interface {
	Save(ctx context.Context, user User) (User, error)
	FindByEmail(email string) bool
	FindByAuthID(id string) (User, error)
}
