package consumer

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/user"

type Consumer struct {
	User *user.User
}

func NewConsumer(auth0ID string, email string, name string, surname string) (*Consumer, error) {
	user, err := user.New(auth0ID, name, surname, email, "consumer")
	if err != nil {
		return nil, err
	}
	return &Consumer{
		User: user,
	}, nil
}
