package steps_test

type conversationCounterpartResponse struct {
	ID           int    `json:"id"`
	Role         string `json:"role"`
	Name         string `json:"name"`
	Surname      string `json:"surname"`
	CategoryName string `json:"category_name,omitempty"`
}
