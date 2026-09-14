package admin

import (
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "admin"

type Admin struct {
	*user.BaseUser
}

func NewAdmin(
	authID string,
	email string,
	name string,
	surname string,
	profilePhoto *filedomain.Image,
) (*Admin, error) {
	baseUser, err := user.New(authID, name, surname, email, Role, profilePhoto)
	if err != nil {
		return nil, err
	}

	return &Admin{BaseUser: baseUser}, nil
}

func RehydrateAdmin(baseUser *user.BaseUser) *Admin {
	return &Admin{BaseUser: baseUser}
}
