package category

import "errors"

var ErrNameRequired = errors.New("Category name is required")

var ErrNameTooLong = errors.New("Category name is too long")

var ErrIDRequired = errors.New("Category id is required")

var ErrAlreadyExists = errors.New("Category already exists")

var ErrDoesNotExist = errors.New("Category does not exist")

var ErrVersionRequired = errors.New("Category expected version must be positive")
var ErrVersionConflict = errors.New("Category version changed")
var ErrNoChanges = errors.New("Category business fields are required")
var ErrInvalidReason = errors.New("Category reason must be 1-500 bytes of printable UTF-8")
var ErrConfirmationRequired = errors.New("Category ongoing orders confirmation is required")
var ErrDisabled = errors.New("Category is disabled")
