package category

type Repository interface {
	Save(category Category) error
	FindByNormalizedName(normalizedName string) bool
}
