package payment_account_handler

type authorizationResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
}

type connectionResponse struct {
	Status                  string `json:"status"`
	AccountID               string `json:"account_id,omitempty"`
	CanReceivePayments      bool   `json:"can_receive_payments"`
	CanSendServiceProposals bool   `json:"can_send_service_proposals"`
}
