package provider

import "errors"

var ErrDoesNotExist = errors.New("Provider does not exist")

var ErrProfileReadersNotConfigured = errors.New("provider profile readers are not configured")

var ErrRatingStatsBatchReaderNotConfigured = errors.New("provider rating stats batch reader is not configured")
