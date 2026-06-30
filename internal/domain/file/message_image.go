package file

type Image struct {
	FileID       string
	OriginalName string
	URL          string
	Description  string
}

type MessageImage = Image

type MessageImageContent struct {
	MessageImage
	MimeType string
	Data     []byte
}
