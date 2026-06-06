package file

type PresignRequest struct {
	AuthID       string
	OriginalName string
	MimeType     string
	SizeBytes    int
	Purpose      string
}

type PresignUploadResult struct {
	FileID  string
	Key     string
	URL     string
	Headers map[string]string
}

type ConfirmRequest struct {
	AuthID    string
	FileID    string
	Key       string
	MimeType  string
	SizeBytes int
}

type ConfirmUploadResult struct {
	FileID       string
	URL          string
	OriginalName string
}
