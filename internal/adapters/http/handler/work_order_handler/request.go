package work_order_handler

type reportCompletionRequest struct {
	Description  string   `json:"description" binding:"required"`
	ImageFileIDs []string `json:"image_file_ids" binding:"required"`
}
