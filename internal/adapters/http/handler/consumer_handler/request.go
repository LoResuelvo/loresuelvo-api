package consumer_handler

type registerConsumerRequest struct {
	Email              string                          `json:"email"`
	Name               string                          `json:"name"`
	Surname            string                          `json:"surname"`
	ProfilePhotoFileID string                          `json:"profile_photo_file_id"`
	Address            *registerConsumerAddressRequest `json:"address"`
}

type registerConsumerAddressRequest struct {
	Street       string `json:"street"`
	StreetNumber string `json:"street_number"`
	Floor        string `json:"floor"`
	Unit         string `json:"unit"`
}
