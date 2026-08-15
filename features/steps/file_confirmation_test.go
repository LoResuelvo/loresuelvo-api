package steps_test

type confirmedFileResponse struct {
	ID           string                     `json:"id"`
	URL          string                     `json:"url"`
	OriginalName string                     `json:"original_name"`
	MimeType     string                     `json:"mime_type"`
	Type         string                     `json:"type"`
	Audio        *confirmedAudioFileDetails `json:"audio"`
	Video        *confirmedVideoFileDetails `json:"video"`
}

type confirmedAudioFileDetails struct {
	Codec           string `json:"codec"`
	DurationSeconds int    `json:"duration_seconds"`
}

type confirmedVideoFileDetails struct {
	VideoCodec      string `json:"video_codec"`
	AudioCodec      string `json:"audio_codec"`
	DurationSeconds int    `json:"duration_seconds"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
}
