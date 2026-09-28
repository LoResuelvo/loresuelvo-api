package readmodel

import "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"

// ConversationAssociation identifies persisted operation and work conversation ownership.
type ConversationAssociation struct {
	ConversationID            int
	ConsumerID                int
	ProviderID                int
	JobRequestID              *int
	ServiceProposalID         *int
	RelatedServiceProposalIDs []int
}

// OperationChat contains the bounded messages prepared for an audited administrative read.
type OperationChat struct {
	OperationID ID
	ConversationAssociation
	Messages []conversation.Message
}

// IsShared reports whether multiple persisted proposals use this conversation.
// Messages remain conversation-owned, never exclusively assigned to the selected proposal.
func (association ConversationAssociation) IsShared() bool {
	return len(association.RelatedServiceProposalIDs) > 1
}
