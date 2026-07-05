package notification

type Type string

const (
	TypeServiceProposalReceived Type = "service_proposal_received"
	TypeServiceProposalAccepted Type = "service_proposal_accepted"
	TypeServiceProposalRejected Type = "service_proposal_rejected"
)

type ResourceType string

const (
	ResourceServiceProposal ResourceType = "service_proposal"
)
