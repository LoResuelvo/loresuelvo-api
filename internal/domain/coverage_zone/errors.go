package coveragezone

import "errors"

var ErrNameRequired = errors.New("Coverage zone name is required")

var ErrDoesNotExist = errors.New("Coverage zone does not exist")

var ErrAtLeastOneRequired = errors.New("At least one coverage zone must be selected")

var ErrNotAvailable = errors.New("Coverage zone is not available")
