package conversation

type Service struct {
	conversationRepository   Repository
	consumerIDFinder         ConsumerIDFinder
	providerExistenceChecker ProviderExistenceChecker
}

func NewService(conversationRepository Repository, consumerIDFinder ConsumerIDFinder, providerExistenceChecker ProviderExistenceChecker) *Service {
	return &Service{
		conversationRepository:   conversationRepository,
		consumerIDFinder:         consumerIDFinder,
		providerExistenceChecker: providerExistenceChecker,
	}
}

func (s *Service) StartWorkRequest(consumerAuthID string, providerID int, content string) (*Conversation, error) {
	consumerID, err := s.consumerIDForWorkRequest(consumerAuthID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureProviderExists(providerID); err != nil {
		return nil, err
	}

	conversation, message, err := newPendingWorkRequest(consumerID, providerID, content)
	if err != nil {
		return nil, err
	}

	if err := s.ensureConversationDoesNotExist(consumerID, providerID); err != nil {
		return nil, err
	}

	return s.conversationRepository.SaveWithMessage(*conversation, *message)
}

func (s *Service) consumerIDForWorkRequest(consumerAuthID string) (int, error) {
	consumerID, err := s.consumerIDFinder.FindIDByAuthID(consumerAuthID)
	if err != nil {
		return 0, ErrOnlyConsumerCanStartWorkRequest
	}

	return consumerID, nil
}

func (s *Service) ensureProviderExists(providerID int) error {
	if providerID <= 0 {
		return ErrProviderRequired
	}

	providerExists, err := s.providerExistenceChecker.ExistsByID(providerID)
	if err != nil {
		return err
	}
	if !providerExists {
		return ErrProviderDoesNotExist
	}

	return nil
}

func (s *Service) ensureConversationDoesNotExist(consumerID, providerID int) error {
	exists, err := s.conversationRepository.ExistsBetween(consumerID, providerID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyExists
	}

	return nil
}

func newPendingWorkRequest(consumerID, providerID int, content string) (*Conversation, *Message, error) {
	conversation, err := NewPendingConversation(consumerID, providerID)
	if err != nil {
		return nil, nil, err
	}

	message, err := NewConsumerMessage(content)
	if err != nil {
		return nil, nil, err
	}

	return conversation, message, nil
}
