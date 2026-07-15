package file

const (
	maxProfilePhotoBytes             = 5 * 1024 * 1024
	maxConversationMessageImageBytes = 5 * 1024 * 1024
	maxJobRequestImageBytes          = 5 * 1024 * 1024
	MaxConversationMessageImages     = 5
	MaxJobRequestImages              = 3
)

type UploadPolicy struct {
	Purpose              string
	Visibility           string
	MaxSizeBytes         int
	AllowedMimeTypes     map[string]struct{}
	InvalidMetadataError error
}

func (policy UploadPolicy) Allows(metadata FileMetadata) bool {
	if metadata.SizeBytes() <= 0 || metadata.SizeBytes() > policy.MaxSizeBytes {
		return false
	}
	_, ok := policy.AllowedMimeTypes[metadata.MimeType()]
	return ok
}

var profilePhotoPolicy = UploadPolicy{
	Purpose:      PurposeProfilePhoto,
	Visibility:   VisibilityPublic,
	MaxSizeBytes: maxProfilePhotoBytes,
	AllowedMimeTypes: map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	},
	InvalidMetadataError: ErrUnsupportedProfilePhoto,
}

var conversationMessageImagePolicy = UploadPolicy{
	Purpose:      PurposeConversationMessageImage,
	Visibility:   VisibilityPrivate,
	MaxSizeBytes: maxConversationMessageImageBytes,
	AllowedMimeTypes: map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	},
	InvalidMetadataError: ErrMessageImageNotAvailable,
}

var jobRequestImagePolicy = UploadPolicy{
	Purpose:      PurposeJobRequestImage,
	Visibility:   VisibilityPrivate,
	MaxSizeBytes: maxJobRequestImageBytes,
	AllowedMimeTypes: map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	},
	InvalidMetadataError: ErrJobRequestImageNotAvailable,
}

func defaultUploadPolicies() map[string]UploadPolicy {
	return map[string]UploadPolicy{
		PurposeProfilePhoto:             profilePhotoPolicy,
		PurposeConversationMessageImage: conversationMessageImagePolicy,
		PurposeJobRequestImage:          jobRequestImagePolicy,
	}
}
