package payment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

const bookingCheckoutValidity = 30 * time.Minute

type Service struct {
	intentRepository      IntentRepository
	serviceProposalFinder ServiceProposalFinder
	userFinder            UserFinder
	paymentAccountFinder  PaymentAccountFinder
	credentialDecryptor   CredentialDecryptor
	checkoutGateway       CheckoutGateway
	paymentVerifier       PaymentVerifier
	paidBookingConfirmer  PaidBookingConfirmer
	notificator           notification.Notificator
	idGenerator           IDGenerator
	clock                 clock.Clock
}

func NewService(
	intentRepository IntentRepository,
	serviceProposalFinder ServiceProposalFinder,
	userFinder UserFinder,
	paymentAccountFinder PaymentAccountFinder,
	credentialDecryptor CredentialDecryptor,
	checkoutGateway CheckoutGateway,
	paymentVerifier PaymentVerifier,
	paidBookingConfirmer PaidBookingConfirmer,
	notificator notification.Notificator,
	idGenerator IDGenerator,
	clock clock.Clock,
) *Service {
	return &Service{
		intentRepository:      intentRepository,
		serviceProposalFinder: serviceProposalFinder,
		userFinder:            userFinder,
		paymentAccountFinder:  paymentAccountFinder,
		credentialDecryptor:   credentialDecryptor,
		checkoutGateway:       checkoutGateway,
		paymentVerifier:       paymentVerifier,
		paidBookingConfirmer:  paidBookingConfirmer,
		notificator:           notificator,
		idGenerator:           idGenerator,
		clock:                 clock,
	}
}

func (service *Service) StartBookingCheckout(ctx context.Context, authID string, proposalID int) (*Intent, error) {
	foundUser, err := service.userFinder.FindByAuthID(authID)
	if err != nil {
		return nil, ErrOnlyProposalRecipientCanCheckout
	}
	proposal, err := service.serviceProposalFinder.FindByID(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal.Consumer.ID() != foundUser.ID() {
		return nil, ErrOnlyProposalRecipientCanCheckout
	}
	if proposal.Status != serviceproposal.StatusPending {
		return nil, ErrProposalNotPending
	}
	if proposal.Provider == nil {
		return nil, fmt.Errorf("starting booking checkout: service proposal provider is required")
	}

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

	now := service.clock.Now().UTC()
	expiresOn := now.Add(bookingCheckoutValidity)
	bookingPaymentDeadline := proposal.BookingTerms.BookingPaymentDeadline().UTC()
	if bookingPaymentDeadline.Before(expiresOn) {
		expiresOn = bookingPaymentDeadline
	}
	if !expiresOn.After(now) {
		return nil, ErrInvalidCheckoutSession
	}
	intent, err := NewBookingDepositIntent(
		service.idGenerator(),
		proposal.ID,
		proposal.BookingTerms,
		now,
	)
	if err != nil {
		return nil, err
	}
	if err := service.intentRepository.Save(ctx, intent); err != nil {
		return nil, err
	}

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
	if err := service.intentRepository.SaveCheckoutReady(ctx, intent); err != nil {
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
	notification PaymentNotification,
) error {
	if strings.TrimSpace(notification.ExternalPaymentID) == "" ||
		strings.TrimSpace(notification.SellerAccountID) == "" {
		return ErrInvalidExternalPayment
	}
	account, err := service.paymentAccountFinder.FindByExternalAccountID(
		ctx,
		notification.SellerAccountID,
		service.checkoutGateway.Provider(),
	)
	if err != nil {
		return fmt.Errorf("finding notified payment account: %w", err)
	}
	accessToken, err := service.credentialDecryptor.Decrypt(account.AccessTokenCiphertext())
	if err != nil {
		return fmt.Errorf("decrypting notified payment credential: %w", err)
	}
	externalPayment, err := service.paymentVerifier.GetPayment(
		ctx,
		accessToken,
		notification.ExternalPaymentID,
	)
	if err != nil {
		return fmt.Errorf("verifying external payment: %w", err)
	}
	if externalPayment.ID != notification.ExternalPaymentID ||
		externalPayment.SellerAccountID != notification.SellerAccountID {
		return ErrInvalidExternalPayment
	}

	intent, err := service.intentRepository.FindByID(ctx, externalPayment.ExternalReference)
	if err != nil {
		return err
	}
	now := service.clock.Now().UTC()
	if externalPayment.Status == ExternalPaymentStatusProcessing {
		if err := intent.MarkProcessing(externalPayment, now); err != nil {
			return err
		}
		if err := service.intentRepository.SaveProcessing(ctx, intent); err != nil {
			return fmt.Errorf("saving processing payment intent: %w", err)
		}
		return nil
	}
	if externalPayment.Status == ExternalPaymentStatusRejected {
		if err := intent.MarkRejected(externalPayment, now); err != nil {
			return err
		}
		if err := service.intentRepository.SaveRejected(ctx, intent); err != nil {
			return fmt.Errorf("saving rejected payment intent: %w", err)
		}
		return nil
	}
	if err := intent.MarkPaid(externalPayment, now); err != nil {
		return err
	}
	proposal, err := service.serviceProposalFinder.FindByID(ctx, intent.ServiceProposalID)
	if err != nil {
		return err
	}
	if err := proposal.Accept(proposal.Consumer.ID(), now); err != nil {
		return err
	}
	order, err := workorder.New(proposal, now)
	if err != nil {
		return err
	}
	acceptedNotification := proposal.CreateAcceptedNotification(service.clock)
	confirmation, err := service.paidBookingConfirmer.ConfirmPaidBooking(
		ctx,
		intent,
		order,
		acceptedNotification,
	)
	if err != nil {
		return err
	}
	if confirmation == nil || confirmation.Notification == nil {
		return fmt.Errorf("processing approved payment: persisted notification is required")
	}

	if err := service.notificator.Notify(ctx, confirmation.Notification); err != nil {
		return fmt.Errorf("notifying provider about accepted service proposal: %w", err)
	}
	return nil
}
