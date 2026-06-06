package file

const maxProviderProfilePhotoBytes = 5 * 1024 * 1024

type UploadPolicy struct {
	Purpose          string
	Visibility       string
	MaxSizeBytes     int
	AllowedMimeTypes map[string]struct{}
}

func (policy UploadPolicy) Allows(metadata FileMetadata) bool {
	if metadata.SizeBytes() <= 0 || metadata.SizeBytes() > policy.MaxSizeBytes {
		return false
	}
	_, ok := policy.AllowedMimeTypes[metadata.MimeType()]
	return ok
}

var providerProfilePhotoPolicy = UploadPolicy{
	Purpose:      PurposeProviderProfilePhoto,
	Visibility:   VisibilityPublic,
	MaxSizeBytes: maxProviderProfilePhotoBytes,
	AllowedMimeTypes: map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	},
}

func defaultUploadPolicies() map[string]UploadPolicy {
	return map[string]UploadPolicy{
		PurposeProviderProfilePhoto: providerProfilePhotoPolicy,
	}
}
