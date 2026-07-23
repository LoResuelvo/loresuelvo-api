package serviceproposal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type Service struct {
	repository               ServiceProposalRepository
	workOrderRepository      WorkOrderRepository
	userRepository           UserRepository
	conversationRepository   ConversationRepository
	notificationRepository   NotificationRepository
	notificator              notification.Notificator
	fileURLResolver          FileURLResolver
	paymentAccountRepository PaymentAccountRepository
	paymentProvider          paymentaccount.PaymentProvider
	bookingPolicy            BookingPolicy
	clock                    clock.Clock
}

func NewService(serviceRepo ServiceProposalRepository, workOrderRepo WorkOrderRepository, userRepo UserRepository, conversationRepo ConversationRepository, notificationRepo NotificationRepository, notificator notification.Notificator, fileURLResolver FileURLResolver, paymentAccountRepository PaymentAccountRepository, paymentProvider paymentaccount.PaymentProvider, bookingPolicy BookingPolicy, clock clock.Clock) *Service {
	return &Service{
		repository:               serviceRepo,
		workOrderRepository:      workOrderRepo,
		userRepository:           userRepo,
		conversationRepository:   conversationRepo,
		notificationRepository:   notificationRepo,
		notificator:              notificator,
		fileURLResolver:          fileURLResolver,
		paymentAccountRepository: paymentAccountRepository,
		paymentProvider:          paymentProvider,
		bookingPolicy:            bookingPolicy,
		clock:                    clock,
	}
}

func (s *Service) CreateServiceProposal(ctx context.Context, auth0ID string, consumerID int, amount int64, scheduledOn time.Time, description string) (*ServiceProposal, error) {
	provider, consumer, conversation, err := s.getParticipants(auth0ID, consumerID)
	if err != nil {
		return nil, err
	}

	paymentAccount, err := s.paymentAccountRepository.FindByProviderID(ctx, provider.ID(), s.paymentProvider)
	if errors.Is(err, paymentaccount.ErrConnectionNotFound) {
		return nil, ErrPaymentAccountConnectionRequired
	}
	if err != nil {
		return nil, fmt.Errorf("finding provider payment account: %w", err)
	}
	if !paymentAccount.CanReceivePayments() {
		return nil, ErrPaymentAccountConnectionRequired
	}

	bookingTerms, err := s.bookingPolicy.Calculate(amount, scheduledOn)
	if err != nil {
		return nil, err
	}

	serviceProposal, err := NewServiceProposal(provider, consumer, conversation, scheduledOn, description, bookingTerms, s.clock)
	if err != nil {
		return nil, err
	}

	savedProposal, err := s.repository.Save(serviceProposal)
	if err != nil {
		return nil, err
	}

	if err := s.saveAndNotify(ctx, savedProposal.CreateReceivedNotification(s.clock)); err != nil {
		return nil, err
	}

	return savedProposal, nil
}

func (s *Service) GetServiceProposals(ctx context.Context, auth0ID string) ([]*ServiceProposal, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, err
	}

	proposals, err := s.repository.FindByUserID(ctx, foundUser.ID())
	if err != nil {
		return nil, err
	}

	fileIDs := make([]string, 0, len(proposals)*2)
	for _, proposal := range proposals {
		for _, participant := range []user.User{proposal.Consumer, proposal.Provider} {
			if participant.ProfilePhoto() != nil {
				fileIDs = append(fileIDs, participant.ProfilePhoto().FileID)
			}
		}
	}

	urlsByFileID, err := s.fileURLResolver.ResolvePublicURLs(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving service proposal counterpart profile photos: %w", err)
	}
	for _, proposal := range proposals {
		for _, participant := range []user.User{proposal.Consumer, proposal.Provider} {
			if participant.ProfilePhoto() != nil {
				participant.SetProfilePhotoURL(urlsByFileID[participant.ProfilePhoto().FileID])
			}
		}
	}
	return proposals, nil
}

func (s *Service) Accept(ctx context.Context, auth0ID string, proposalID int) (*workorder.WorkOrder, error) {
	foundUser, err := s.userRepository.FindByAuthID(auth0ID)
	if err != nil {
		return nil, ErrOnlyRecipientCanAccept
	}

	proposal, err := s.repository.FindByID(ctx, proposalID)
	if err != nil {
		return nil, err
	}

	acceptedOn := s.clock.Now()
	if err := proposal.Accept(foundUser.ID(), acceptedOn); err != nil {
		return nil, err
	}

	order, err := workorder.New(proposal, acceptedOn)
	if err != nil {
		return nil, err
	}

	savedOrder, err := s.workOrderRepository.Save(ctx, order)
	if err != nil {
		return nil, err
	}

	if err := s.saveAndNotify(ctx, proposal.CreateAcceptedNotification(s.clock)); err != nil {
		return nil, err
	}
	return savedOrder, nil
}

func (s *Service) saveAndNotify(ctx context.Context, createdNotification *notification.Notification) error {
	savedNotification, err := s.notificationRepository.Save(ctx, createdNotification)
	if err != nil {
		return err
	}
	if s.notificator != nil {
		if err := s.notificator.Notify(ctx, savedNotification); err != nil {
			return err
		}
	}
	return nil
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

	conversation, err := s.conversationRepository.FindBetween(consumer.ID(), provider.ID())
	if err != nil {
		return nil, nil, nil, ErrConversationRequired
	}

	return provider, consumer, conversation, nil
}
