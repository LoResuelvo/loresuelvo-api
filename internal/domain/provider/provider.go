package provider

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type Provider struct {
	User     *user.User
	Category *category.Category
}

func NewProvider(auth0ID string, email string, name string, surname string, providerCategory *category.Category) (*Provider, error) {
	if providerCategory == nil {
		return nil, category.ErrDoesNotExist
	}

	user, err := user.New(auth0ID, name, surname, email, "provider")
	if err != nil {
		return nil, err
	}

	return &Provider{
		User:     user,
		Category: providerCategory,
	}, nil
}
