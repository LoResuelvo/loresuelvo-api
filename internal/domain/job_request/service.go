package jobrequest

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request/read_model"
)

type Service struct {
	repository             Repository
	consumerRepository     ConsumerRepository
	providerRepository     ProviderRepository
	conversationRepository ConversationRepository
}

func NewService(
	repository Repository,
	consumerRepository ConsumerRepository,
	providerRepository ProviderRepository,
	conversationRepository ConversationRepository,
) *Service {
	return &Service{
		repository:             repository,
		consumerRepository:     consumerRepository,
		providerRepository:     providerRepository,
		conversationRepository: conversationRepository,
	}
}

func (s *Service) Create(consumerAuthID string, providerID int, title, description string) (*JobRequest, error) {
	consumerID, err := s.consumerIDForJobRequest(consumerAuthID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureProviderExists(providerID); err != nil {
		return nil, err
	}

	jobRequest, err := New(consumerID, providerID, title, description)
	if err != nil {
		return nil, err
	}

	pendingConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	if err != nil {
		return nil, err
	}

	return s.repository.SaveWithConversation(*jobRequest, *pendingConversation)
}

func (s *Service) GetJobRequests(userAuthID string) ([]readmodel.JobRequestSummary, error) {
	return s.repository.FindByUserAuthID(userAuthID)
}

func (s *Service) consumerIDForJobRequest(consumerAuthID string) (int, error) {
	consumerID, err := s.consumerRepository.FindIDByAuthID(consumerAuthID)
	if err != nil {
		return 0, ErrOnlyConsumerCanCreateJobRequest
	}

	return consumerID, nil
}

func (s *Service) ensureProviderExists(providerID int) error {
	if providerID <= 0 {
		return ErrProviderRequired
	}

	exists, err := s.providerRepository.ExistsByID(providerID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrProviderDoesNotExist
	}

	return nil
}
