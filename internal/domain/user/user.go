package user

import (
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/validator"
)

type User interface {
	ID() int
	AuthID() string
	Email() string
	Name() string
	Surname() string
	Role() string
	ProfilePhoto() *filedomain.Image
	SetPersistenceID(id int)
	SetProfilePhotoURL(url string)
}

type BaseUser struct {
	id           int
	authID       string
	name         string
	surname      string
	email        string
	role         string
	profilePhoto *filedomain.Image
}

func New(authID, name, surname, email, role string, profilePhoto *filedomain.Image) (*BaseUser, error) {
	if !validator.ValidateEmail(email) {
		return nil, validator.ErrInvalidEmailFormat
	}

	return &BaseUser{
		authID:       authID,
		name:         name,
		surname:      surname,
		email:        email,
		role:         role,
		profilePhoto: profilePhoto,
	}, nil
}

func RehydrateBaseUser(id int, authID, email, name, surname, role string, profilePhoto *filedomain.Image) *BaseUser {
	return &BaseUser{id: id, authID: authID, email: email, name: name, surname: surname, role: role, profilePhoto: profilePhoto}
}

func (user *BaseUser) ID() int {
	return user.id
}

func (user *BaseUser) AuthID() string {
	return user.authID
}

func (user *BaseUser) Email() string {
	return user.email
}

func (user *BaseUser) Name() string {
	return user.name
}

func (user *BaseUser) Surname() string {
	return user.surname
}

func (user *BaseUser) Role() string {
	return user.role
}

func (user *BaseUser) ProfilePhoto() *filedomain.Image {
	return user.profilePhoto
}

func (user *BaseUser) SetPersistenceID(id int) {
	user.id = id
}

func (user *BaseUser) SetProfilePhotoURL(url string) {
	if user.profilePhoto != nil {
		user.profilePhoto.URL = url
	}
}
