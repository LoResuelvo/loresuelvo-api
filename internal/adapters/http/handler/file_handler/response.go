package file_handler

type presignFileResponse struct {
	FileID    string            `json:"file_id"`
	Key       string            `json:"key"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
}

type fileResponse struct {
	ID           string             `json:"id"`
	URL          string             `json:"url,omitempty"`
	OriginalName string             `json:"original_name"`
	MimeType     string             `json:"mime_type"`
	Type         string             `json:"type"`
	Audio        *fileAudioResponse `json:"audio,omitempty"`
	Video        *fileVideoResponse `json:"video,omitempty"`
}

type fileAudioResponse struct {
	Codec           string `json:"codec"`
	DurationSeconds int    `json:"duration_seconds"`
}

type fileVideoResponse struct {
	VideoCodec      string `json:"video_codec"`
	AudioCodec      string `json:"audio_codec,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
}
