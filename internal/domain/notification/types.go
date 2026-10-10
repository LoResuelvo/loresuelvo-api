package notification

type Type string

const (
	TypeJobRequestReceived              Type = "job_request_received"
	TypeWorkOrderFinalPaymentApproved   Type = "work_order_final_payment_approved"
	TypeServiceProposalReceived         Type = "service_proposal_received"
	TypeServiceProposalAccepted         Type = "service_proposal_accepted"
	TypeServiceProposalRejected         Type = "service_proposal_rejected"
	TypeWorkOrderCloseToScheduledTime   Type = "work_order_close_to_scheduled_time"
	TypeWorkOrderCompletionReported     Type = "work_order_completion_reported"
	TypeCalendarReauthorizationRequired Type = "calendar_reauthorization_required"
)

type ResourceType string

const (
	ResourceJobRequest         ResourceType = "job_request"
	ResourceServiceProposal    ResourceType = "service_proposal"
	ResourceWorkOrder          ResourceType = "work_order"
	ResourceCalendarConnection ResourceType = "calendar_connection"
)
