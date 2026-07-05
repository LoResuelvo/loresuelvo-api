package consumer

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/user"

const Role = "consumer"

type Consumer struct {
	*user.BaseUser
}

func NewConsumer(auth0ID string, email string, name string, surname string) (*Consumer, error) {
	baseUser, err := user.New(auth0ID, name, surname, email, Role)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		BaseUser: baseUser,
	}, nil
}
