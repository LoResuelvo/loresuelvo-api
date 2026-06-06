package file

import "errors"

var (
	ErrFileIDRequired           = errors.New("File id is required")
	ErrFileKeyRequired          = errors.New("File key is required")
	ErrFileBucketRequired       = errors.New("File bucket is required")
	ErrFileTimestampRequired    = errors.New("File timestamp is required")
	ErrFileStatusRequired       = errors.New("File status is required")
	ErrOriginalNameRequired     = errors.New("Original name is required")
	ErrMimeTypeRequired         = errors.New("Mime type is required")
	ErrSizeRequired             = errors.New("File size is required")
	ErrVisibilityRequired       = errors.New("File visibility is required")
	ErrPurposeRequired          = errors.New("File purpose is required")
	ErrUploaderRequired         = errors.New("File uploader is required")
	ErrUnsupportedPurpose       = errors.New("File purpose is not supported")
	ErrUnsupportedProfilePhoto  = errors.New("Profile photo could not be uploaded")
	ErrProfilePhotoRequired     = errors.New("Profile photo is required")
	ErrProfilePhotoNotAvailable = errors.New("Profile photo could not be uploaded")
)
