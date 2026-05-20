package user

type Repository interface {
	Save(user User) error
	FindByEmail(email string) bool
	FindByAuthID(id string) (*User, error)
}
