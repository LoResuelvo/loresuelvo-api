package conversation

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/category"

type RecommendationCategoryLister interface {
	ListAll() ([]category.Category, error)
}
