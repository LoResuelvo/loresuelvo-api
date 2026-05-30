package consumer

type Repository interface {
	Save(consumer Consumer) error
	FindByEmail(email string) bool
}

type ConsumerIDFinder interface {
	FindIDByAuthID(authID string) (int, error)
}
