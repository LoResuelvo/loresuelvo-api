package file

import "time"

const (
	StatusPending   = "pending"
	StatusConfirmed = "confirmed"

	VisibilityPublic  = "public"
	VisibilityPrivate = "private"

	PurposeProviderProfilePhoto     = "provider_profile_photo"
	PurposeConversationMessageImage = "conversation_message_image"
	PurposeJobRequestImage          = "job_request_image"
)

type File struct {
	ID               string
	Key              string
	Bucket           string
	metadata         FileMetadata
	Status           string
	Visibility       string
	Purpose          string
	UploadedByAuthID string
	CreatedOn        time.Time
	UpdatedOn        time.Time
}

func NewPendingFile(id, key, bucket string, metadata FileMetadata, visibility, purpose, uploadedByAuthID string, createdOn time.Time) (*File, error) {
	file, err := NewFile(id, key, bucket, metadata, StatusPending, visibility, purpose, uploadedByAuthID, createdOn, createdOn)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func NewFile(id, key, bucket string, metadata FileMetadata, status, visibility, purpose, uploadedByAuthID string, createdOn, updatedOn time.Time) (*File, error) {
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

func (f File) Metadata() FileMetadata {
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
