package provider

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "provider"

type Provider struct {
	*user.BaseUser
	Category *category.Category
}

func NewProvider(auth0ID string, email string, name string, surname string, providerCategory *category.Category, profilePhotoFileID string) (*Provider, error) {
	if providerCategory == nil {
		return nil, category.ErrDoesNotExist
	}

	providerUser, err := user.New(auth0ID, name, surname, email, Role, profilePhotoFileID)
	if err != nil {
		return nil, err
	}

	return &Provider{
		BaseUser: providerUser,
		Category: providerCategory,
	}, nil
}

func (p Provider) AuthID() string {
	return p.BaseUser.AuthID
}

func (p Provider) Email() string {
	return p.BaseUser.Email
}

func (p Provider) Name() string {
	return p.BaseUser.Name
}

func (p Provider) Surname() string {
	return p.BaseUser.Surname
}

func (p Provider) HasCategory(categoryID int) bool {
	return categoryID > 0 && p.Category != nil && p.Category.ID == categoryID
}
