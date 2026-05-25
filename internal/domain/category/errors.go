package category

import "errors"

var ErrNameRequired = errors.New("Category name is required")

var ErrAlreadyExists = errors.New("Category already exists")
