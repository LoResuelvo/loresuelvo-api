package provider

type Repository interface {
	Save(provider Provider) error
	FindByEmail(email string) bool
	FindByCategoryID(categoryID int) ([]Provider, error)
}
