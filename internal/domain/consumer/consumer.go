package consumer

type Consumer struct {
	Auth0ID string
	Email   string
	Name    string
	Surname string
}

func NewConsumer(auth0ID string, email string, name string, surname string) Consumer {
	return Consumer{
		Auth0ID: auth0ID,
		Email:   email,
		Name:    name,
		Surname: surname,
	}
}
