package provider

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "provider"

type Provider struct {
	ID                 int
	User               *user.User
	Category           *category.Category
	ProfilePhotoFileID string
}

func NewProvider(auth0ID string, email string, name string, surname string, providerCategory *category.Category, profilePhotoFileID string) (*Provider, error) {
	if providerCategory == nil {
		return nil, category.ErrDoesNotExist
	}

	providerUser, err := user.New(auth0ID, name, surname, email, Role)
	if err != nil {
		return nil, err
	}

	return &Provider{
		User:               providerUser,
		Category:           providerCategory,
		ProfilePhotoFileID: profilePhotoFileID,
	}, nil
}

func (p Provider) AuthID() string {
	return p.User.AuthID
}

func (p Provider) Email() string {
	return p.User.Email
}

func (p Provider) Name() string {
	return p.User.Name
}

func (p Provider) Surname() string {
	return p.User.Surname
}

func (p Provider) HasCategory(categoryID int) bool {
	return categoryID > 0 && p.Category != nil && p.Category.ID == categoryID
}
