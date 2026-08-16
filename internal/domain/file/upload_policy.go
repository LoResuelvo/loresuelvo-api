package file

const (
	maxProfilePhotoBytes                       = 5 * 1024 * 1024
	maxConversationMessageImageBytes           = 5 * 1024 * 1024
	maxConversationMessageAudioBytes           = 5 * 1024 * 1024
	maxConversationMessageVideoBytes           = 50 * 1024 * 1024
	maxJobRequestImageBytes                    = 5 * 1024 * 1024
	maxWorkOrderCompletionImageBytes           = 5 * 1024 * 1024
	MaxConversationMessageImages               = 5
	MaxJobRequestImages                        = 3
	MaxWorkOrderCompletionImages               = 3
	MaxConversationMessageAudioDurationSeconds = 300
	MaxConversationMessageVideoDurationSeconds = 120
	MaxConversationMessageVideoWidth           = 1920
	MaxConversationMessageVideoHeight          = 1920
)

type UploadPolicy struct {
	Purpose              string
	Visibility           string
	MaxSizeBytes         int
	AllowedMimeTypes     map[string]struct{}
	AllowedCodecs        map[string]struct{}
	AllowedVideoCodecs   map[string]struct{}
	AllowedAudioCodecs   map[string]struct{}
	MaxDurationSeconds   int
	MaxWidth             int
	MaxHeight            int
	InvalidMetadataError error
}

func (policy UploadPolicy) Allows(metadata Metadata) bool {
	if metadata.SizeBytes() <= 0 || metadata.SizeBytes() > policy.MaxSizeBytes {
		return false
	}
	_, ok := policy.AllowedMimeTypes[metadata.MimeType()]
	return ok
}

func (policy UploadPolicy) AllowsConfirmedAudio(metadata Metadata) bool {
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

func (policy UploadPolicy) AllowsConfirmedVideo(metadata Metadata) bool {
	if !policy.Allows(metadata) {
		return false
	}
	if policy.MaxDurationSeconds > 0 && (metadata.DurationSeconds() <= 0 || metadata.DurationSeconds() > policy.MaxDurationSeconds) {
		return false
	}
	if policy.MaxWidth > 0 && metadata.Width() > policy.MaxWidth {
		return false
	}
	if policy.MaxHeight > 0 && metadata.Height() > policy.MaxHeight {
		return false
	}
	if len(policy.AllowedVideoCodecs) > 0 {
		if _, ok := policy.AllowedVideoCodecs[metadata.VideoCodec()]; !ok {
			return false
		}
	}
	if metadata.AudioCodec() != "" && len(policy.AllowedAudioCodecs) > 0 {
		if _, ok := policy.AllowedAudioCodecs[metadata.AudioCodec()]; !ok {
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

var conversationMessageVideoPolicy = UploadPolicy{
	Purpose:            PurposeConversationMessageVideo,
	Visibility:         VisibilityPrivate,
	MaxSizeBytes:       maxConversationMessageVideoBytes,
	MaxDurationSeconds: MaxConversationMessageVideoDurationSeconds,
	MaxWidth:           MaxConversationMessageVideoWidth,
	MaxHeight:          MaxConversationMessageVideoHeight,
	AllowedMimeTypes: map[string]struct{}{
		"video/mp4": {},
	},
	AllowedVideoCodecs: map[string]struct{}{
		"h264": {},
	},
	AllowedAudioCodecs: map[string]struct{}{
		"aac": {},
	},
	InvalidMetadataError: ErrUnsupportedMessageVideo,
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

var workOrderCompletionImagePolicy = UploadPolicy{
	Purpose:      PurposeWorkOrderCompletionImage,
	Visibility:   VisibilityPrivate,
	MaxSizeBytes: maxWorkOrderCompletionImageBytes,
	AllowedMimeTypes: map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
	},
	InvalidMetadataError: ErrWorkOrderCompletionImageNotAvailable,
}

func defaultUploadPolicies() map[string]UploadPolicy {
	return map[string]UploadPolicy{
		PurposeProfilePhoto:             profilePhotoPolicy,
		PurposeConversationMessageImage: conversationMessageImagePolicy,
		PurposeConversationMessageAudio: conversationMessageAudioPolicy,
		PurposeConversationMessageVideo: conversationMessageVideoPolicy,
		PurposeJobRequestImage:          jobRequestImagePolicy,
		PurposeWorkOrderCompletionImage: workOrderCompletionImagePolicy,
	}
}
