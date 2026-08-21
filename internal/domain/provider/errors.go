package provider

import "errors"

var ErrDoesNotExist = errors.New("Provider does not exist")

var ErrProfileReaderNotConfigured = errors.New("provider profile reader is not configured")

var ErrCoverageZoneFinderNotConfigured = errors.New("provider coverage zone finder is not configured")
