package job_request_handler

type createJobRequestRequest struct {
	ProviderID   int      `json:"provider_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	ImageFileIDs []string `json:"image_file_ids"`
}

type createJobRequestFromChatbotRequest struct {
	ProviderID int `json:"provider_id"`
}
