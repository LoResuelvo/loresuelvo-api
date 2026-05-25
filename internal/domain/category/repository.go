package category

type Repository interface {
	Save(category Category) (*Category, error)
	FindByID(id int) *Category
	FindByNormalizedName(normalizedName string) *Category
}
