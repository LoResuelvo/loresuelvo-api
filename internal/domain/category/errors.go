package category

import "errors"

var ErrNameRequired = errors.New("Category name is required")

var ErrIDRequired = errors.New("Category id is required")

var ErrAlreadyExists = errors.New("Category already exists")

var ErrDoesNotExist = errors.New("Category does not exist")
