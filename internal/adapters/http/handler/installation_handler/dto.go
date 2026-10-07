package installation_handler

type registrationRequest struct {
	Secret            string `json:"installation_secret"`
	App               string `json:"app"`
	Token             string `json:"fcm_token"`
	Locale            string `json:"locale"`
	BindingID         string `json:"binding_id"`
	PreviousBindingID string `json:"previous_binding_id"`
}
type removalRequest struct {
	Secret    string `json:"installation_secret"`
	BindingID string `json:"binding_id"`
}
type registrationResponse struct {
	ID        string `json:"installation_id"`
	BindingID string `json:"binding_id"`
	App       string `json:"app"`
	Locale    string `json:"locale"`
	Enabled   bool   `json:"enabled"`
}
