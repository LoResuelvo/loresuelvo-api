package admin_handler

import "time"

type consumerDirectoryResponse struct {
	ID              int       `json:"id"`
	Role            string    `json:"role"`
	Name            string    `json:"name"`
	Surname         string    `json:"surname"`
	Email           string    `json:"email"`
	ProfilePhotoURL *string   `json:"profile_photo_url"`
	CreatedOn       time.Time `json:"created_on"`
}
