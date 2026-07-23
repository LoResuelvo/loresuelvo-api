package payment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
)

const bookingCheckoutValidity = 30 * time.Minute

type Service struct {
	intentRepository      IntentRepository
	serviceProposalFinder ServiceProposalFinder
	userFinder            UserFinder
	paymentAccountFinder  PaymentAccountFinder
	credentialDecryptor   CredentialDecryptor
	checkoutGateway       CheckoutGateway
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
