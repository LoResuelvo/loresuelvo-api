package file

type MessageImage struct {
	FileID       string
	OriginalName string
	URL          string
}

type MessageImageContent struct {
	MessageImage
	MimeType string
	Data     []byte
}
