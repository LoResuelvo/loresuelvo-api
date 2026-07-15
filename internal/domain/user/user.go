package user

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"

type User interface {
	Base() *BaseUser
}

type BaseUser struct {
	AuthID             string
	ID                 int
	Name               string
	Surname            string
	Email              string
	Role               string
	ProfilePhotoFileID string
}

func New(authID, name, surname, email, role, profilePhotoFileID string) (*BaseUser, error) {
	if !validator.ValidateEmail(email) {
		return nil, validator.ErrInvalidEmailFormat
	}

	return &BaseUser{
		AuthID:             authID,
		Name:               name,
		Surname:            surname,
		Email:              email,
		Role:               role,
		ProfilePhotoFileID: profilePhotoFileID,
	}, nil
}

func (user *BaseUser) Base() *BaseUser {
	return user
}
