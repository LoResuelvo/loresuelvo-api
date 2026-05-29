package conversation

const StatusPending = "pending"

type Conversation struct {
	ID         int
	ConsumerID int
	ProviderID int
	Status     string
}

func NewPendingConversation(consumerID, providerID int) (*Conversation, error) {
	if consumerID <= 0 {
		return nil, ErrOnlyConsumerCanStartWorkRequest
	}

	if providerID <= 0 {
		return nil, ErrProviderRequired
	}

	return &Conversation{
		ConsumerID: consumerID,
		ProviderID: providerID,
		Status:     StatusPending,
	}, nil
}
