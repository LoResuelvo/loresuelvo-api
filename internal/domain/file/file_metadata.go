package file

type FileMetadata struct {
	originalName string
	mimeType     string
	sizeBytes    int
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

func (m FileMetadata) OriginalName() string {
	return m.originalName
}

func (m FileMetadata) MimeType() string {
	return m.mimeType
}

func (m FileMetadata) SizeBytes() int {
	return m.sizeBytes
}
