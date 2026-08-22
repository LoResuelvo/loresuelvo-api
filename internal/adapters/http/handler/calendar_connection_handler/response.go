package calendar_connection_handler

type authorizationResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
}

type connectionRequest struct {
	ServerAuthCode string `json:"server_auth_code" binding:"required"`
}
