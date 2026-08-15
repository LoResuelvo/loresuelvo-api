package file

import "time"

const (
	StatusPending   = "pending"
	StatusConfirmed = "confirmed"

	VisibilityPublic  = "public"
	VisibilityPrivate = "private"

	PurposeProfilePhoto             = "profile_photo"
	PurposeConversationMessageImage = "conversation_message_image"
	PurposeConversationMessageAudio = "conversation_message_audio"
	PurposeConversationMessageVideo = "conversation_message_video"
	PurposeJobRequestImage          = "job_request_image"
)

type File struct {
	ID               string
	Key              string
	Bucket           string
	metadata         Metadata
	Status           string
	Visibility       string
	Purpose          string
	UploadedByAuthID string
	CreatedOn        time.Time
	UpdatedOn        time.Time
}

func NewPendingFile(id, key, bucket string, metadata Metadata, visibility, purpose, uploadedByAuthID string, createdOn time.Time) (*File, error) {
	file, err := NewFile(id, key, bucket, metadata, StatusPending, visibility, purpose, uploadedByAuthID, createdOn, createdOn)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func NewFile(id, key, bucket string, metadata Metadata, status, visibility, purpose, uploadedByAuthID string, createdOn, updatedOn time.Time) (*File, error) {
	if metadata == nil {
		return nil, ErrFileMetadataRequired
	}
	if err := validateFileFields(id, status, key, bucket, visibility, purpose, uploadedByAuthID, createdOn, updatedOn); err != nil {
		return nil, err
	}

	return &File{
		ID:               id,
		Key:              key,
		Bucket:           bucket,
		metadata:         metadata,
		Status:           status,
		Visibility:       visibility,
		Purpose:          purpose,
		UploadedByAuthID: uploadedByAuthID,
		CreatedOn:        createdOn,
		UpdatedOn:        updatedOn,
	}, nil
}

func (f *File) Confirm(updatedOn time.Time) {
	f.Status = StatusConfirmed
	f.UpdatedOn = updatedOn
}

func (f File) OriginalName() string {
	return f.metadata.OriginalName()
}

func (f File) MimeType() string {
	return f.metadata.MimeType()
}

func (f File) SizeBytes() int {
	return f.metadata.SizeBytes()
}

func (f File) DurationSeconds() int {
	return f.metadata.DurationSeconds()
}

func (f File) Codec() string {
	return f.metadata.Codec()
}

func (f File) VideoCodec() string {
	return f.metadata.VideoCodec()
}

func (f File) AudioCodec() string {
	return f.metadata.AudioCodec()
}

func (f File) Width() int {
	return f.metadata.Width()
}

func (f File) Height() int {
	return f.metadata.Height()
}

func (f File) Metadata() Metadata {
	return f.metadata
}

func (f File) IsConfirmed() bool {
	return f.Status == StatusConfirmed
}

func (f File) IsPublic() bool {
	return f.Visibility == VisibilityPublic
}

func (f File) WasUploadedBy(authID string) bool {
	return f.UploadedByAuthID == authID
}

func (f File) HasPurpose(purpose string) bool {
	return f.Purpose == purpose
}

func (f File) IsAudio() bool {
	return f.HasPurpose(PurposeConversationMessageAudio)
}

func (f File) IsVideo() bool {
	return f.HasPurpose(PurposeConversationMessageVideo)
}

func (f *File) ConfirmAudio(updatedOn time.Time, durationSeconds int, codec string) error {
	metadata, err := NewAudioFileMetadata(f.OriginalName(), f.MimeType(), f.SizeBytes(), durationSeconds, codec)
	if err != nil {
		return err
	}

	f.metadata = metadata
	f.Confirm(updatedOn)
	return nil
}

func (f *File) ConfirmVideo(updatedOn time.Time, metadata VideoMetadata) error {
	fileMetadata, err := NewVideoFileMetadata(f.OriginalName(), f.MimeType(), f.SizeBytes(), metadata)
	if err != nil {
		return err
	}

	f.metadata = fileMetadata
	f.Confirm(updatedOn)
	return nil
}

func validateFileFields(id, status, key, bucket, visibility, purpose, uploadedByAuthID string, createdOn, updatedOn time.Time) error {
	if id == "" {
		return ErrFileIDRequired
	}
	if status == "" {
		return ErrFileStatusRequired
	}
	if key == "" {
		return ErrFileKeyRequired
	}
	if bucket == "" {
		return ErrFileBucketRequired
	}
	if visibility == "" {
		return ErrVisibilityRequired
	}
	if purpose == "" {
		return ErrPurposeRequired
	}
	if uploadedByAuthID == "" {
		return ErrUploaderRequired
	}
	if createdOn.IsZero() {
		return ErrFileTimestampRequired
	}
	if updatedOn.IsZero() {
		return ErrFileTimestampRequired
	}

	return nil
}
