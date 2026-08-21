package coveragezone

import "errors"

var ErrNameRequired = errors.New("Coverage zone name is required")

var ErrMarketRequired = errors.New("Coverage zone market is required")

var ErrCodeRequired = errors.New("Coverage zone code is required")

var ErrKindRequired = errors.New("Coverage zone kind is required")

var ErrKindInvalid = errors.New("Coverage zone kind is invalid")

var ErrExternalProviderRequired = errors.New("Coverage zone external provider is required")

var ErrExternalIDRequired = errors.New("Coverage zone external id is required")

var ErrCoverageZoneRequired = errors.New("Coverage zone is required")

var ErrDoesNotExist = errors.New("Coverage zone does not exist")

var ErrAtLeastOneRequired = errors.New("At least one coverage zone must be selected")

var ErrNotAvailable = errors.New("Coverage zone is not available")

var ErrDuplicateCoverageZone = errors.New("Coverage zone cannot be selected more than once")
