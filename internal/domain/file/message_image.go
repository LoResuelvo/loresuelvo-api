package file

type Image struct {
	FileID       string
	OriginalName string
	URL          string
}

type MessageImage struct {
	Image
	Description string
}

type MessageImageContent struct {
	MessageImage
	MimeType string
	Data     []byte
}
