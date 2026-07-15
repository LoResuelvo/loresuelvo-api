package user

import (
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type User interface {
	Base() *BaseUser
}

type BaseUser struct {
	AuthID       string
	ID           int
	Name         string
	Surname      string
	Email        string
	Role         string
	ProfilePhoto *filedomain.Image
}

func New(authID, name, surname, email, role string, profilePhoto *filedomain.Image) (*BaseUser, error) {
	if !validator.ValidateEmail(email) {
		return nil, validator.ErrInvalidEmailFormat
	}

	return &BaseUser{
		AuthID:       authID,
		Name:         name,
		Surname:      surname,
		Email:        email,
		Role:         role,
		ProfilePhoto: profilePhoto,
	}, nil
}

func (user *BaseUser) Base() *BaseUser {
	return user
}
