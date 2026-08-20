package coveragezone

import "errors"

var ErrNameRequired = errors.New("Coverage zone name is required")

var ErrDoesNotExist = errors.New("Coverage zone does not exist")
