package serviceproposal

import (
	"context"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal/read_model"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type Service struct {
	repository             ServiceProposalRepository
	userRepository         UserRepository
	conversationRepository ConversationRepository
	notificationRepository NotificationRepository
	notificator            notification.Notificator
	fileURLResolver        FileURLResolver
	clock                  clock.Clock
}

func NewService(serviceRepo ServiceProposalRepository, userRepo UserRepository, conversationRepo ConversationRepository, notificationRepo NotificationRepository, notificator notification.Notificator, fileURLResolver FileURLResolver, clock clock.Clock) *Service {
	return &Service{
		repository:             serviceRepo,
		userRepository:         userRepo,
		conversationRepository: conversationRepo,
		notificationRepository: notificationRepo,
		notificator:            notificator,
		fileURLResolver:        fileURLResolver,
		clock:                  clock,
	}
}

func (s *Service) CreateServiceProposal(ctx context.Context, auth0ID string, consumerID int, amount int64, scheduledOn time.Time, description string) (*ServiceProposal, error) {
	provider, consumer, conversation, err := s.getParticipants(auth0ID, consumerID)
	if err != nil {
		return nil, err
	}

	serviceProposal, err := NewServiceProposal(provider, consumer, conversation, amount, scheduledOn, description, s.clock)
	if err != nil {
		return nil, err
	}

	savedProposal, err := s.repository.Save(serviceProposal)
	if err != nil {
		return nil, err
	}

	notification := savedProposal.CreateNotification(s.clock)
	savedNotification, err := s.notificationRepository.Save(ctx, notification)
	if err != nil {
		return nil, err
	}

	if s.notificator != nil {
		if err := s.notificator.Notify(ctx, savedNotification); err != nil {
			return nil, err
		}
	}

	return savedProposal, nil
}

func (s *Service) GetServiceProposals(ctx context.Context, auth0ID string) ([]readmodel.ServiceProposalSummary, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, err
	}

	proposals, err := s.repository.FindByUserID(ctx, foundUser.Base().ID)
	if err != nil {
		return nil, err
	}

	summaries := make([]readmodel.ServiceProposalSummary, 0, len(proposals))
	fileIDs := make([]string, 0, len(proposals))
	for _, proposal := range proposals {
		summary, err := serviceProposalSummaryFor(proposal, foundUser.Base().Role)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
		if summary.Counterpart.ProfilePhotoFileID != "" {
			fileIDs = append(fileIDs, summary.Counterpart.ProfilePhotoFileID)
		}
	}

	urlsByFileID, err := s.fileURLResolver.ResolvePublicURLs(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving service proposal counterpart profile photos: %w", err)
	}
	for index := range summaries {
		summaries[index].Counterpart.ProfilePhotoURL = urlsByFileID[summaries[index].Counterpart.ProfilePhotoFileID]
	}
	return summaries, nil
}

func (s *Service) Accept(ctx context.Context, auth0ID string, proposalID int) (*ServiceProposal, *workorder.WorkOrder, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, nil, ErrOnlyRecipientCanAccept
	}

	proposal, err := s.repository.FindByID(ctx, proposalID)
	if err != nil {
		return nil, nil, err
	}

	order, err := proposal.Accept(foundUser.Base().ID, s.clock.Now())
	if err != nil {
		return nil, nil, err
	}

	savedOrder, err := s.repository.SaveWithWorkOrder(ctx, proposal, order)
	if err != nil {
		return nil, nil, err
	}
	return proposal, savedOrder, nil
}

func serviceProposalSummaryFor(proposal *ServiceProposal, viewerRole string) (readmodel.ServiceProposalSummary, error) {
	if proposal == nil || proposal.Conversation == nil {
		return readmodel.ServiceProposalSummary{}, fmt.Errorf("mapping service proposal summary: incomplete proposal")
	}
	summary := readmodel.ServiceProposalSummary{
		ID:             proposal.ID,
		ConversationID: proposal.Conversation.Base().ID,
		Amount:         proposal.Amount,
		ScheduledOn:    proposal.ScheduledOn,
		Description:    proposal.Description,
		Status:         string(proposal.Status),
		CreatedOn:      proposal.CreatedOn,
	}
	switch viewerRole {
	case consumer.Role:
		if proposal.Provider == nil || proposal.Provider.Category == nil {
			return readmodel.ServiceProposalSummary{}, fmt.Errorf("mapping service proposal summary: incomplete provider counterpart")
		}
		summary.Counterpart = readmodel.Counterpart{
			ID:                 proposal.Provider.ID,
			Role:               provider.Role,
			Name:               proposal.Provider.Base().Name,
			Surname:            proposal.Provider.Base().Surname,
			CategoryName:       proposal.Provider.Category.Name,
			ProfilePhotoFileID: proposal.Provider.ProfilePhotoFileID,
		}
	case provider.Role:
		if proposal.Consumer == nil {
			return readmodel.ServiceProposalSummary{}, fmt.Errorf("mapping service proposal summary: incomplete consumer counterpart")
		}
		summary.Counterpart = readmodel.Counterpart{
			ID:      proposal.Consumer.ID,
			Role:    consumer.Role,
			Name:    proposal.Consumer.Base().Name,
			Surname: proposal.Consumer.Base().Surname,
		}
	default:
		return readmodel.ServiceProposalSummary{}, fmt.Errorf("mapping service proposal summary: unsupported viewer role %q", viewerRole)
	}
	return summary, nil
}

func (s *Service) getParticipants(providerAuth0ID string, consumerID int) (*provider.Provider, *consumer.Consumer, conversation.Conversation, error) {
	provider, err := s.userRepository.FindProviderByAuthID(providerAuth0ID)
	if err != nil {
		return nil, nil, nil, ErrProviderRequired
	}

	consumer, err := s.userRepository.FindConsumerByID(consumerID)
	if err != nil {
		return nil, nil, nil, ErrConsumerRequired
	}

	conversation, err := s.conversationRepository.FindBetween(consumer.Base().ID, provider.Base().ID)
	if err != nil {
		return nil, nil, nil, ErrConversationRequired
	}

	return provider, consumer, conversation, nil
}
