package category_test

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/mock"
)

type categoryRepositoryMock struct {
	mock.Mock
}

func (repository *categoryRepositoryMock) Save(categoryToSave category.Category) (*category.Category, error) {
	arguments := repository.Called(categoryToSave)
	savedCategory, _ := arguments.Get(0).(*category.Category)
	return savedCategory, arguments.Error(1)
}

func (repository *categoryRepositoryMock) ListAll() ([]category.Category, error) {
	arguments := repository.Called()
	categories, _ := arguments.Get(0).([]category.Category)
	return categories, arguments.Error(1)
}

func (repository *categoryRepositoryMock) FindByID(id int) *category.Category {
	arguments := repository.Called(id)
	foundCategory, _ := arguments.Get(0).(*category.Category)
	return foundCategory
}
