package consumer

import "context"

type Repository interface {
	Save(consumer Consumer) error
	FindByEmail(email string) bool
}

type ConsumerIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
}

type ConsumerFinder interface {
	FindByIDs(ctx context.Context, ids []int) ([]Consumer, error)
}
