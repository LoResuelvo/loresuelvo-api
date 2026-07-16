package consumer

import (
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const Role = "consumer"

type Consumer struct {
	*user.BaseUser
}

func NewConsumer(auth0ID, email, name, surname string, profilePhoto *filedomain.Image) (*Consumer, error) {
	baseUser, err := user.New(auth0ID, name, surname, email, Role, profilePhoto)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		BaseUser: baseUser,
	}, nil
}
