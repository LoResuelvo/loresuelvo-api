package conversation_handler

type sendMessageRequest struct {
	Content      string   `json:"content"`
	ImageFileIDs []string `json:"image_file_ids"`
}

type createChatbotConversationRequest struct {
	Content      string   `json:"content"`
	ImageFileIDs []string `json:"image_file_ids"`
}
