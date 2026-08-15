package file

// VideoMetadataParser inspects uploaded video bytes without owning storage or transport concerns.
type VideoMetadataParser interface {
	Parse(data []byte) (VideoMetadata, error)
}
