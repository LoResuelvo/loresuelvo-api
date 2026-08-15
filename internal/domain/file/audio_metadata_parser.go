package file

// AudioMetadata is the validated technical metadata extracted from an audio object.
type AudioMetadata struct {
	DurationSeconds int
	Codec           string
}

// AudioMetadataParser inspects uploaded audio bytes without owning storage or transport concerns.
type AudioMetadataParser interface {
	Parse(data []byte) (AudioMetadata, error)
}
