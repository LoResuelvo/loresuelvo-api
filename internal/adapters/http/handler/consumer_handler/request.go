package consumer_handler

type registerConsumerRequest struct {
	Email              string `json:"email"`
	Name               string `json:"name"`
	Surname            string `json:"surname"`
	ProfilePhotoFileID string `json:"profile_photo_file_id"`
}
