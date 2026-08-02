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

const checkoutValidity = 30 * time.Minute

type CheckoutResult struct {
	Intent  *Intent
	Created bool
}

type Service struct {
	intentRepository          IntentRepository
	transactionRepository     TransactionRepository
	serviceProposalFinder     ServiceProposalFinder
	workOrderFinder           WorkOrderFinder
	userFinder                UserFinder
	paymentAccountFinder      PaymentAccountFinder
	lockManager               LockManager
	unitOfWork                UnitOfWork
	secretProtector           SecretProtector
	checkoutGateway           CheckoutGateway
	paymentVerifier           PaymentVerifier
	notificator               notification.Notificator
	idGenerator               IDGenerator
	confirmationCodeGenerator ConfirmationCodeGenerator
	clock                     clock.Clock
	checkoutPolicy            BookingCheckoutPolicy
	serviceBalancePolicy      ServiceBalanceCheckoutPolicy
}

func NewService(
	intentRepository IntentRepository,
	transactionRepository TransactionRepository,
	serviceProposalFinder ServiceProposalFinder,
	workOrderFinder WorkOrderFinder,
	userFinder UserFinder,
	paymentAccountFinder PaymentAccountFinder,
	lockManager LockManager,
	unitOfWork UnitOfWork,
	secretProtector SecretProtector,
	checkoutGateway CheckoutGateway,
	paymentVerifier PaymentVerifier,
	notificator notification.Notificator,
	idGenerator IDGenerator,
	confirmationCodeGenerator ConfirmationCodeGenerator,
	clock clock.Clock,
) *Service {
	return &Service{
		intentRepository:          intentRepository,
		transactionRepository:     transactionRepository,
		serviceProposalFinder:     serviceProposalFinder,
		workOrderFinder:           workOrderFinder,
		userFinder:                userFinder,
		paymentAccountFinder:      paymentAccountFinder,
		lockManager:               lockManager,
		unitOfWork:                unitOfWork,
		secretProtector:           secretProtector,
		checkoutGateway:           checkoutGateway,
		paymentVerifier:           paymentVerifier,
		notificator:               notificator,
		idGenerator:               idGenerator,
		confirmationCodeGenerator: confirmationCodeGenerator,
		clock:                     clock,
	}
}

func (service *Service) StartBookingCheckout(
	ctx context.Context,
	authID string,
	proposalID int,
) (*CheckoutResult, error) {
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

	var result *CheckoutResult
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
				result = &CheckoutResult{Intent: intent}
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
		result = &CheckoutResult{Intent: created, Created: true}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *Service) StartServiceBalanceCheckout(
	ctx context.Context,
	authID string,
	workOrderID int,
) (*CheckoutResult, error) {
	foundUser, err := service.userFinder.FindByAuthID(authID)
	if err != nil {
		return nil, ErrOnlyWorkOrderConsumerCanCheckout
	}
	order, err := service.workOrderFinder.FindByID(ctx, workOrderID)
	if err != nil {
		return nil, err
	}
	now := service.clock.Now().UTC()
	if err := service.serviceBalancePolicy.Authorize(order, foundUser.ID(), now); err != nil {
		return nil, err
	}

	var result *CheckoutResult
	err = service.lockManager.WithinLock(ctx, ServiceBalanceCheckoutLockKey(workOrderID), func() error {
		lockedOrder, findOrderErr := service.workOrderFinder.FindByID(ctx, workOrderID)
		if findOrderErr != nil {
			return findOrderErr
		}
		now = service.clock.Now().UTC()
		if authorizeErr := service.serviceBalancePolicy.Authorize(lockedOrder, foundUser.ID(), now); authorizeErr != nil {
			return authorizeErr
		}
		intent, findIntentErr := service.intentRepository.FindLatestByProposalIDAndPurpose(
			ctx,
			lockedOrder.ServiceProposalID(),
			PurposeServiceBalance,
		)
		if findIntentErr == nil {
			reuse, prepareErr := intent.PrepareCheckout(now)
			if prepareErr != nil {
				return prepareErr
			}
			if reuse {
				result = &CheckoutResult{Intent: intent}
				return nil
			}
			if intent.Status == StatusExpired {
				if saveErr := service.intentRepository.Save(ctx, intent); saveErr != nil {
					return saveErr
				}
			}
		} else if !errors.Is(findIntentErr, ErrIntentDoesNotExist) {
			return findIntentErr
		}

		intent, createIntentErr := NewServiceBalanceIntent(service.idGenerator(), lockedOrder, now)
		if createIntentErr != nil {
			return createIntentErr
		}
		created, createCheckoutErr := service.createCheckout(
			ctx,
			foundUser,
			lockedOrder.ProviderID(),
			intent,
			now,
			now.Add(checkoutValidity),
		)
		if createCheckoutErr != nil {
			return createCheckoutErr
		}
		result = &CheckoutResult{Intent: created, Created: true}
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
	intent, err := NewBookingDepositIntent(service.idGenerator(), proposal.ID, proposal.BookingTerms, now)
	if err != nil {
		return nil, err
	}
	expiresOn := service.checkoutPolicy.Expiration(proposal.BookingTerms, now, checkoutValidity)
	return service.createCheckout(ctx, foundUser, proposal.Provider.ID(), intent, now, expiresOn)
}

func (service *Service) createCheckout(
	ctx context.Context,
	foundUser user.User,
	providerID int,
	intent *Intent,
	now time.Time,
	expiresOn time.Time,
) (*Intent, error) {
	account, err := service.paymentAccountFinder.FindByProviderID(
		ctx,
		providerID,
		service.checkoutGateway.Provider(),
	)
	if err != nil {
		return nil, fmt.Errorf("finding provider payment account: %w", err)
	}
	if !account.CanReceivePayments() {
		return nil, fmt.Errorf("starting checkout: provider payment account cannot receive payments")
	}
	accessToken, err := service.secretProtector.Decrypt(account.AccessTokenCiphertext())
	if err != nil {
		return nil, fmt.Errorf("decrypting provider payment credential: %w", err)
	}
	if strings.TrimSpace(accessToken) == "" {
		return nil, fmt.Errorf("decrypting provider payment credential: decrypted credential is empty")
	}
	if err := service.intentRepository.Save(ctx, intent); err != nil {
		return nil, err
	}
	externalCheckout, err := service.checkoutGateway.CreateCheckout(ctx, accessToken, CheckoutRequest{
		ExternalReference: intent.ID,
		Purpose:           intent.Purpose,
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
	accessToken, err := service.secretProtector.Decrypt(account.AccessTokenCiphertext())
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

func (persistence *paymentOutcomePersistence) VisitServiceBalanceApproved(outcome ServiceBalanceApproved) error {
	order, err := persistence.service.workOrderFinder.FindByServiceProposalID(
		persistence.ctx,
		outcome.Intent.ServiceProposalID,
	)
	if err != nil {
		return err
	}
	code, err := persistence.service.confirmationCodeGenerator.Generate()
	if err != nil {
		return fmt.Errorf("generating work order confirmation code: %w", err)
	}
	codeCiphertext, err := persistence.service.secretProtector.Encrypt(code.String())
	if err != nil {
		return fmt.Errorf("encrypting work order confirmation code: %w", err)
	}
	issuedOn := persistence.service.clock.Now().UTC()
	authorization, err := workorder.NewCompletionAuthorization(codeCiphertext, issuedOn)
	if err != nil {
		return err
	}
	if err := order.CompletePayment(authorization); err != nil {
		return err
	}
	return persistence.service.unitOfWork.Execute(
		persistence.ctx,
		func(store TransactionalStore) error {
			if err := store.SaveTransaction(persistence.ctx, outcome.Transaction); err != nil {
				return err
			}
			if err := store.SaveIntent(persistence.ctx, outcome.Intent); err != nil {
				return err
			}
			return store.SaveWorkOrder(persistence.ctx, order)
		},
	)
}
