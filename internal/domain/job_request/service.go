package jobrequest

import (
	"context"
	"errors"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
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

func (s *Service) Create(consumerAuthID string, providerID int, title, description string, images []string) (*JobRequest, error) {
	consumerID, err := s.consumerIDForJobRequest(consumerAuthID)
	if err != nil {
		return nil, err
	}

	if err := s.ensureProviderExists(providerID); err != nil {
		return nil, err
	}

	if err := s.ensureNoOpenJobRequest(consumerID, providerID); err != nil {
		return nil, err
	}

	pendingConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	if err != nil {
		return nil, err
	}

	jobRequest, err := New(consumerID, providerID, title, description)
	if err != nil {
		return nil, err
	}

	return s.repository.SaveWithConversation(*jobRequest, pendingConversation)
}

func (s *Service) CreateFromChatbotAssessment(ctx context.Context, consumerAuthID string, chatbotConversationID, providerID int) (*JobRequest, error) {
	consumerID, err := s.consumerIDForJobRequest(consumerAuthID)
	if err != nil {
		return nil, err
	}
	foundConversation, err := s.conversationRepository.FindByID(ctx, chatbotConversationID)
	if err != nil {
		return nil, err
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok || chatbotConversation.ConsumerID != consumerID {
		return nil, ErrChatbotConversationAccessDenied
	}
	assessment := chatbotConversation.CurrentAssessment
	if assessment == nil || assessment.Outcome == conversation.AssessmentCollectingInformation {
		return nil, ErrAssessmentNeedsMoreInformation
	}
	if !assessment.RequiresProfessional() || assessment.ProblemCategoryID == nil {
		return nil, ErrAssessmentNotContactable
	}
	foundProvider, err := s.providerRepository.FindByID(ctx, providerID)
	if err != nil {
		if errors.Is(err, provider.ErrDoesNotExist) {
			return nil, ErrProviderDoesNotExist
		}
		return nil, err
	}
	if !foundProvider.HasCategory(*assessment.ProblemCategoryID) {
		return nil, ErrProviderCategoryMismatch
	}
	if err := s.ensureNoOpenJobRequest(consumerID, providerID); err != nil {
		return nil, err
	}
	jobRequest, err := NewFromAssessment(consumerID, providerID, *assessment)
	if err != nil {
		return nil, err
	}
	pendingConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	if err != nil {
		return nil, err
	}
	return s.repository.SaveWithConversation(*jobRequest, pendingConversation)
}

func (s *Service) GetJobRequests(userAuthID string) ([]readmodel.JobRequestSummary, error) {
	return s.repository.FindByUserAuthID(userAuthID)
}

func (s *Service) Accept(ctx context.Context, providerAuthID string, jobRequestID int) (*JobRequest, error) {
	jobRequest, err := s.repository.FindByID(jobRequestID)
	if err != nil {
		return nil, err
	}

	providerID, err := s.providerRepository.FindIDByAuthID(providerAuthID)
	if err != nil {
		return nil, ErrOnlyAssignedProviderCanAcceptJobRequest
	}

	if err := jobRequest.Accept(providerID); err != nil {
		return nil, err
	}

	linkedConversation, err := s.conversationRepository.FindByID(ctx, jobRequest.ConversationID)
	if err != nil {
		return nil, err
	}

	if err := linkedConversation.Activate(); err != nil {
		return nil, err
	}

	if err := s.conversationRepository.SaveStatus(ctx, linkedConversation); err != nil {
		return nil, err
	}

	if err := s.repository.SaveStatus(ctx, *jobRequest); err != nil {
		return nil, err
	}

	return jobRequest, nil
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

func (s *Service) ensureNoOpenJobRequest(consumerID, providerID int) error {
	exists, err := s.repository.ExistsBetweenWithAnyStatus(consumerID, providerID, OpenStatuses())
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyExists
	}

	return nil
}
