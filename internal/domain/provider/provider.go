package provider

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "provider"

type Provider struct {
	*user.BaseUser
	Category *category.Category
}

func NewProvider(auth0ID string, email string, name string, surname string, providerCategory *category.Category, profilePhoto *filedomain.Image) (*Provider, error) {
	if providerCategory == nil {
		return nil, category.ErrDoesNotExist
	}

	providerUser, err := user.New(auth0ID, name, surname, email, Role, profilePhoto)
	if err != nil {
		return nil, err
	}

	return &Provider{
		BaseUser: providerUser,
		Category: providerCategory,
	}, nil
}

func (p Provider) Categoryname() string {
	return p.Category.Name
}

func (p Provider) HasCategory(categoryID int) bool {
	return p.Category.ID == categoryID
}
