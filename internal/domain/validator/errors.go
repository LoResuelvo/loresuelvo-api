package validator

import "errors"

var ErrInvalidEmailFormat = errors.New("Invalid email format")

var ErrEmailAlreadyRegistered = errors.New("Email is already registered")
