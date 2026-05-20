package user

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"

type User struct {
	AuthID  string
	Name    string
	Surname string
	Email   string
	Role    string
}

func New(authID, name, surname, email, role string) (*User, error) {
	if !validator.ValidateEmail(email) {
		return nil, validator.ErrInvalidEmailFormat
	}

	return &User{
		AuthID:  authID,
		Name:    name,
		Surname: surname,
		Email:   email,
		Role:    role,
	}, nil
}
