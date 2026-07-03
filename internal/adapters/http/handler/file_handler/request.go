package file_handler

type presignFileRequest struct {
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int    `json:"size_bytes"`
	Purpose      string `json:"purpose"`
}

type confirmFileRequest struct {
	Key       string `json:"key"`
	MimeType  string `json:"mime_type"`
	SizeBytes int    `json:"size_bytes"`
}
