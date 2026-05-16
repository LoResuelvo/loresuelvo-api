package consumer

type Repository interface {
	Save(consumer Consumer) error
	FindByEmail(email string) bool
}
