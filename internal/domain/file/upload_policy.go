package file

const (
	maxProfilePhotoBytes                       = 5 * 1024 * 1024
	maxConversationMessageImageBytes           = 5 * 1024 * 1024
	maxConversationMessageAudioBytes           = 5 * 1024 * 1024
	maxJobRequestImageBytes                    = 5 * 1024 * 1024
	MaxConversationMessageImages               = 5
	MaxJobRequestImages                        = 3
	MaxConversationMessageAudioDurationSeconds = 300
)

type UploadPolicy struct {
	Purpose              string
	Visibility           string
	MaxSizeBytes         int
	AllowedMimeTypes     map[string]struct{}
	AllowedCodecs        map[string]struct{}
	MaxDurationSeconds   int
	InvalidMetadataError error
}

func (policy UploadPolicy) Allows(metadata FileMetadata) bool {
	if metadata.SizeBytes() <= 0 || metadata.SizeBytes() > policy.MaxSizeBytes {
		return false
	}
	_, ok := policy.AllowedMimeTypes[metadata.MimeType()]
	return ok
}

func (policy UploadPolicy) AllowsConfirmedAudio(metadata FileMetadata) bool {
	if !policy.Allows(metadata) {
		return false
	}
	if policy.MaxDurationSeconds > 0 && (metadata.DurationSeconds() <= 0 || metadata.DurationSeconds() > policy.MaxDurationSeconds) {
		return false
	}
	if len(policy.AllowedCodecs) > 0 {
		_, ok := policy.AllowedCodecs[metadata.Codec()]
		if !ok {
			return false
		}
	}
	return true
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

var conversationMessageAudioPolicy = UploadPolicy{
	Purpose:            PurposeConversationMessageAudio,
	Visibility:         VisibilityPrivate,
	MaxSizeBytes:       maxConversationMessageAudioBytes,
	MaxDurationSeconds: MaxConversationMessageAudioDurationSeconds,
	AllowedMimeTypes: map[string]struct{}{
		"audio/webm": {},
	},
	AllowedCodecs: map[string]struct{}{
		"opus": {},
	},
	InvalidMetadataError: ErrUnsupportedMessageAudio,
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
		PurposeConversationMessageAudio: conversationMessageAudioPolicy,
		PurposeJobRequestImage:          jobRequestImagePolicy,
	}
}
