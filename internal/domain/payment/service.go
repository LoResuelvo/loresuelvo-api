package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

const bookingCheckoutValidity = 30 * time.Minute

type BookingCheckoutResult struct {
	Intent  *Intent
	Created bool
}

type Service struct {
	intentRepository      IntentRepository
	transactionRepository TransactionRepository
	serviceProposalFinder ServiceProposalFinder
	userFinder            UserFinder
	paymentAccountFinder  PaymentAccountFinder
	lockManager           LockManager
	unitOfWork            UnitOfWork
	credentialDecryptor   CredentialDecryptor
	checkoutGateway       CheckoutGateway
	paymentVerifier       PaymentVerifier
	notificator           notification.Notificator
	idGenerator           IDGenerator
	clock                 clock.Clock
	checkoutPolicy        BookingCheckoutPolicy
}

func NewService(
	intentRepository IntentRepository,
	transactionRepository TransactionRepository,
	serviceProposalFinder ServiceProposalFinder,
	userFinder UserFinder,
	paymentAccountFinder PaymentAccountFinder,
	lockManager LockManager,
	unitOfWork UnitOfWork,
	credentialDecryptor CredentialDecryptor,
	checkoutGateway CheckoutGateway,
	paymentVerifier PaymentVerifier,
	notificator notification.Notificator,
	idGenerator IDGenerator,
	clock clock.Clock,
) *Service {
	return &Service{
		intentRepository:      intentRepository,
		transactionRepository: transactionRepository,
		serviceProposalFinder: serviceProposalFinder,
		userFinder:            userFinder,
		paymentAccountFinder:  paymentAccountFinder,
		lockManager:           lockManager,
		unitOfWork:            unitOfWork,
		credentialDecryptor:   credentialDecryptor,
		checkoutGateway:       checkoutGateway,
		paymentVerifier:       paymentVerifier,
		notificator:           notificator,
		idGenerator:           idGenerator,
		clock:                 clock,
	}
}

func (service *Service) StartBookingCheckout(
	ctx context.Context,
	authID string,
	proposalID int,
) (*BookingCheckoutResult, error) {
	foundUser, err := service.userFinder.FindByAuthID(authID)
	if err != nil {
		return nil, ErrOnlyProposalRecipientCanCheckout
	}
	proposal, err := service.serviceProposalFinder.FindByID(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if err := service.checkoutPolicy.Authorize(proposal, foundUser.ID(), service.clock.Now()); err != nil {
		return nil, err
	}

	var result *BookingCheckoutResult
	err = service.lockManager.WithinLock(ctx, BookingCheckoutLockKey(proposalID), func() error {
		now := service.clock.Now().UTC()
		if err := service.checkoutPolicy.Authorize(proposal, foundUser.ID(), now); err != nil {
			return err
		}
		intent, findErr := service.intentRepository.FindLatestByProposalIDAndPurpose(
			ctx,
			proposalID,
			PurposeBookingDeposit,
		)
		if findErr == nil {
			reuse, prepareErr := intent.PrepareCheckout(now)
			if prepareErr != nil {
				return prepareErr
			}
			if reuse {
				intent.BookingTerms = proposal.BookingTerms
				result = &BookingCheckoutResult{Intent: intent}
				return nil
			}
			if intent.Status == StatusExpired {
				if saveErr := service.intentRepository.Save(ctx, intent); saveErr != nil {
					return saveErr
				}
			}
		} else if !errors.Is(findErr, ErrIntentDoesNotExist) {
			return findErr
		}

		created, createErr := service.createBookingCheckout(ctx, foundUser, proposal, now)
		if createErr != nil {
			return createErr
		}
		result = &BookingCheckoutResult{Intent: created, Created: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *Service) createBookingCheckout(
	ctx context.Context,
	foundUser user.User,
	proposal *serviceproposal.ServiceProposal,
	now time.Time,
) (*Intent, error) {
	account, err := service.paymentAccountFinder.FindByProviderID(
		ctx,
		proposal.Provider.ID(),
		service.checkoutGateway.Provider(),
	)
	if err != nil {
		return nil, fmt.Errorf("finding provider payment account: %w", err)
	}
	if !account.CanReceivePayments() {
		return nil, fmt.Errorf("starting booking checkout: provider payment account cannot receive payments")
	}
	accessToken, err := service.credentialDecryptor.Decrypt(account.AccessTokenCiphertext())
	if err != nil {
		return nil, fmt.Errorf("decrypting provider payment credential: %w", err)
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, fmt.Errorf("decrypting provider payment credential: decrypted credential is empty")
	}

	intent, err := NewBookingDepositIntent(service.idGenerator(), proposal.ID, proposal.BookingTerms, now)
	if err != nil {
		return nil, err
	}
	if err := service.intentRepository.Save(ctx, intent); err != nil {
		return nil, err
	}
	expiresOn := service.checkoutPolicy.Expiration(proposal.BookingTerms, now, bookingCheckoutValidity)
	externalCheckout, err := service.checkoutGateway.CreateCheckout(ctx, accessToken, CheckoutRequest{
		ExternalReference: intent.ID,
		Currency:          intent.Currency,
		SellerAmountCents: intent.SellerAmountCents,
		PlatformFeeCents:  intent.PlatformFeeCents,
		TotalAmountCents:  intent.TotalAmountCents,
		PayerEmail:        foundUser.Email(),
		StartsOn:          now,
		ExpiresOn:         expiresOn,
	})
	if err != nil {
		return nil, fmt.Errorf("creating external checkout: %w", err)
	}
	if err := intent.MarkCheckoutReady(
		externalCheckout.ID,
		externalCheckout.URL,
		expiresOn,
		service.clock.Now(),
	); err != nil {
		return nil, err
	}
	if err := service.intentRepository.Save(ctx, intent); err != nil {
		return nil, err
	}
	return intent, nil
}

func (service *Service) GetIntent(ctx context.Context, authID, intentID string) (*Intent, error) {
	foundUser, err := service.userFinder.FindByAuthID(authID)
	if err != nil {
		return nil, ErrOnlyProposalRecipientCanView
	}
	intent, err := service.intentRepository.FindByID(ctx, intentID)
	if err != nil {
		return nil, err
	}
	proposal, err := service.serviceProposalFinder.FindByID(ctx, intent.ServiceProposalID)
	if err != nil {
		return nil, err
	}
	if proposal.Consumer == nil || proposal.Consumer.ID() != foundUser.ID() {
		return nil, ErrOnlyProposalRecipientCanView
	}
	return intent, nil
}

func (service *Service) ProcessPaymentNotification(
	ctx context.Context,
	event PaymentNotification,
) error {
	if strings.TrimSpace(event.ExternalPaymentID) == "" ||
		strings.TrimSpace(event.SellerAccountID) == "" {
		return ErrInvalidExternalPayment
	}
	account, err := service.paymentAccountFinder.FindByExternalAccountID(
		ctx,
		event.SellerAccountID,
		service.checkoutGateway.Provider(),
	)
	if err != nil {
		return fmt.Errorf("finding notified payment account: %w", err)
	}
	accessToken, err := service.credentialDecryptor.Decrypt(account.AccessTokenCiphertext())
	if err != nil {
		return fmt.Errorf("decrypting notified payment credential: %w", err)
	}
	external, err := service.paymentVerifier.GetPayment(ctx, accessToken, event.ExternalPaymentID)
	if err != nil {
		return fmt.Errorf("verifying external payment: %w", err)
	}
	verified, err := NewVerifiedPayment(external)
	if err != nil {
		return err
	}
	if verified.ExternalID() != event.ExternalPaymentID ||
		verified.SellerAccountID() != event.SellerAccountID {
		return ErrInvalidExternalPayment
	}

	return service.lockManager.WithinLock(
		ctx,
		ExternalPaymentLockKey(service.checkoutGateway.Provider(), verified.ExternalID()),
		func() error {
			return service.applyVerifiedPayment(ctx, verified)
		},
	)
}

func (service *Service) applyVerifiedPayment(ctx context.Context, verified VerifiedPayment) error {
	_, err := service.transactionRepository.FindByExternalID(
		ctx,
		service.checkoutGateway.Provider(),
		verified.ExternalID(),
	)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrTransactionDoesNotExist) {
		return fmt.Errorf("finding external payment transaction: %w", err)
	}
	intent, err := service.intentRepository.FindByID(ctx, verified.IntentID())
	if err != nil {
		return err
	}
	outcome, err := verified.ApplyTo(intent, service.checkoutGateway.Provider(), service.clock.Now().UTC())
	if err != nil {
		return err
	}
	return outcome.Accept(&paymentOutcomePersistence{service: service, ctx: ctx})
}

type paymentOutcomePersistence struct {
	service *Service
	ctx     context.Context
}

func (persistence *paymentOutcomePersistence) VisitIntentUpdated(outcome IntentUpdated) error {
	if err := persistence.service.intentRepository.Save(persistence.ctx, outcome.Intent); err != nil {
		return fmt.Errorf("saving updated payment intent: %w", err)
	}
	return nil
}

func (persistence *paymentOutcomePersistence) VisitBookingApproved(outcome BookingApproved) error {
	proposal, err := persistence.service.serviceProposalFinder.FindByID(
		persistence.ctx,
		outcome.Intent.ServiceProposalID,
	)
	if err != nil {
		return err
	}
	now := persistence.service.clock.Now().UTC()
	if err := proposal.Accept(proposal.Consumer.ID(), now); err != nil {
		return err
	}
	order, err := workorder.New(proposal, now)
	if err != nil {
		return err
	}
	acceptedNotification := proposal.CreateAcceptedNotification(persistence.service.clock)
	err = persistence.service.unitOfWork.Execute(
		persistence.ctx,
		func(store TransactionalStore) error {
			if err := store.SaveTransaction(persistence.ctx, outcome.Transaction); err != nil {
				return err
			}
			if err := store.SaveIntent(persistence.ctx, outcome.Intent); err != nil {
				return err
			}
			if err := store.SaveServiceProposal(persistence.ctx, proposal); err != nil {
				return err
			}
			if err := store.SaveWorkOrder(persistence.ctx, order); err != nil {
				return err
			}
			return store.SaveNotification(persistence.ctx, acceptedNotification)
		},
	)
	if err != nil {
		return err
	}
	if err := persistence.service.notificator.Notify(persistence.ctx, acceptedNotification); err != nil {
		return fmt.Errorf("notifying provider about accepted service proposal: %w", err)
	}
	return nil
}
