package file_handler

type presignFileResponse struct {
	FileID    string            `json:"file_id"`
	Key       string            `json:"key"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
}

type fileResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url,omitempty"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type,omitempty"`
	Codec           string `json:"codec,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}
