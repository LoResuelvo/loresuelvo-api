package work_order_handler

type reportCompletionRequest struct {
	Description  string   `json:"description" binding:"required"`
	ImageFileIDs []string `json:"image_file_ids" binding:"required"`
}

type createReviewRequest struct {
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}
