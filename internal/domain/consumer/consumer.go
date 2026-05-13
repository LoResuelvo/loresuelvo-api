package consumer

type Consumer struct {
	Email    string
	Name     string
	Surname  string
	Password string
}

func NewConsumer(email string, name string, surname string, password string) Consumer {
	return Consumer{
		Email:    email,
		Name:     name,
		Surname:  surname,
		Password: password,
	}
}
