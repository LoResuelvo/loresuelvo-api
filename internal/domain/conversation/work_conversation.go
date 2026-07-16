package conversation

type WorkConversation struct {
	*BaseConversation
	ProviderID int
	ConsumerID int
}

func NewPendingConversation(consumerID, providerID int) (Conversation, error) {
	if consumerID <= 0 {
		return nil, ErrConsumerRequired
	}

	if providerID <= 0 {
		return nil, ErrProviderRequired
	}

	return &WorkConversation{
		BaseConversation: NewBaseConversation(TypeWork, StatusPending),
		ConsumerID:       consumerID,
		ProviderID:       providerID,
	}, nil
}
