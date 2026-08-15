package file

import "strings"

type FileMetadata struct {
	originalName    string
	mimeType        string
	sizeBytes       int
	durationSeconds int
	codec           string
}

func NewFileMetadata(originalName, mimeType string, sizeBytes int) (*FileMetadata, error) {
	if originalName == "" {
		return nil, ErrOriginalNameRequired
	}
	if mimeType == "" {
		return nil, ErrMimeTypeRequired
	}
	if sizeBytes <= 0 {
		return nil, ErrSizeRequired
	}

	return &FileMetadata{
		originalName: originalName,
		mimeType:     mimeType,
		sizeBytes:    sizeBytes,
	}, nil
}

func NewAudioFileMetadata(originalName, mimeType string, sizeBytes, durationSeconds int, codec string) (*FileMetadata, error) {
	metadata, err := NewFileMetadata(originalName, mimeType, sizeBytes)
	if err != nil {
		return nil, err
	}
	if err := metadata.SetAudioMetadata(durationSeconds, codec); err != nil {
		return nil, err
	}

	return metadata, nil
}

func (m FileMetadata) OriginalName() string {
	return m.originalName
}

func (m FileMetadata) MimeType() string {
	return m.mimeType
}

func (m FileMetadata) SizeBytes() int {
	return m.sizeBytes
}

func (m FileMetadata) DurationSeconds() int {
	return m.durationSeconds
}

func (m FileMetadata) Codec() string {
	return m.codec
}

func (m *FileMetadata) SetAudioMetadata(durationSeconds int, codec string) error {
	if durationSeconds <= 0 {
		return ErrAudioDurationRequired
	}

	normalizedCodec := strings.ToLower(strings.TrimSpace(codec))
	if normalizedCodec == "" {
		return ErrAudioCodecRequired
	}

	m.durationSeconds = durationSeconds
	m.codec = normalizedCodec
	return nil
}
