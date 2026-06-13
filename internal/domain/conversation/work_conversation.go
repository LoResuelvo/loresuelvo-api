package conversation

type WorkConversation struct {
	*BaseConversation
	ProviderID int
	ConsumerID int
}

func NewPendingConversation(consumerID, providerID int) (Conversation, error) {
	if consumerID <= 0 {
		return nil, ErrOnlyConsumerCanStartWorkRequest
	}

	if providerID <= 0 {
		return nil, ErrProviderRequired
	}

	return &WorkConversation{
		BaseConversation: &BaseConversation{
			Type:   TypeWork,
			Status: StatusPending,
		},
		ConsumerID: consumerID,
		ProviderID: providerID,
	}, nil
}
