package consumer

type Repository interface {
	Save(consumer Consumer) error
}
