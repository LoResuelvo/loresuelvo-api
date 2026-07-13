package notification

type Type string

const (
	TypeServiceProposalReceived       Type = "service_proposal_received"
	TypeServiceProposalAccepted       Type = "service_proposal_accepted"
	TypeServiceProposalRejected       Type = "service_proposal_rejected"
	TypeWorkOrderCloseToScheduledTime Type = "work_order_close_to_scheduled_time"
)

type ResourceType string

const (
	ResourceServiceProposal ResourceType = "service_proposal"
	ResourceWorkOrder       ResourceType = "work_order"
)
