package user

type User struct {
	AuthID  string
	Name    string
	Surname string
	Email   string
	Role    string
}

func New(authID, name, surname, email, role string) *User {
	return &User{
		AuthID:  authID,
		Name:    name,
		Surname: surname,
		Email:   email,
		Role:    role,
	}
}
