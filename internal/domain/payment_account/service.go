package paymentaccount

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

const authorizationAttemptLifetime = 10 * time.Minute

type Service struct {
	userRepository                 UserRepository
	authorizationAttemptRepository AuthorizationAttemptRepository
	paymentAccountRepository       PaymentAccountRepository
	oauthConnector                 OAuthConnector
	credentialProtector            CredentialProtector
	secretGenerator                SecretGenerator
	clock                          Clock
}

func NewService(userRepository UserRepository, authorizationAttemptRepository AuthorizationAttemptRepository, paymentAccountRepository PaymentAccountRepository, oauthConnector OAuthConnector, credentialProtector CredentialProtector, secretGenerator SecretGenerator, clock Clock) *Service {
	return &Service{
		userRepository:                 userRepository,
		authorizationAttemptRepository: authorizationAttemptRepository,
		paymentAccountRepository:       paymentAccountRepository,
		oauthConnector:                 oauthConnector,
		credentialProtector:            credentialProtector,
		secretGenerator:                secretGenerator,
		clock:                          clock,
	}
}

func (service *Service) StartAuthorization(ctx context.Context, authID string) (*Authorization, error) {
	providerID, err := service.authenticatedProviderID(authID)
	if err != nil {
		return nil, err
	}
	if err := service.ensurePaymentAccountIsNotConnected(ctx, providerID); err != nil {
		return nil, err
	}

	state, err := service.secretGenerator.Generate()
	if err != nil {
		return nil, fmt.Errorf("generating authorization state: %w", err)
	}
	codeVerifier, err := service.secretGenerator.Generate()
	if err != nil {
		return nil, fmt.Errorf("generating PKCE verifier: %w", err)
	}
	codeVerifierCiphertext, err := service.credentialProtector.Encrypt(codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("protecting PKCE verifier: %w", err)
	}
	authorizationURL, err := service.oauthConnector.AuthorizationURL(state, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("building payment account authorization URL: %w", err)
	}

	attempt := NewAuthorizationAttempt(
		providerID,
		service.oauthConnector.Provider(),
		stateDigest(state),
		codeVerifierCiphertext,
		service.clock.Now().Add(authorizationAttemptLifetime),
	)
	if err := service.authorizationAttemptRepository.Save(ctx, attempt); err != nil {
		return nil, fmt.Errorf("saving payment account authorization attempt: %w", err)
	}

	return &Authorization{URL: authorizationURL, State: state}, nil
}

func (service *Service) ensurePaymentAccountIsNotConnected(ctx context.Context, providerID int) error {
	_, err := service.paymentAccountRepository.FindByProviderID(ctx, providerID, service.oauthConnector.Provider())
	if err == nil {
		return ErrAlreadyConnected
	}
	if errors.Is(err, ErrConnectionNotFound) {
		return nil
	}
	return fmt.Errorf("checking existing payment account connection: %w", err)
}

func (service *Service) CompleteAuthorization(ctx context.Context, state, code string) (*PaymentAccount, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		return nil, ErrAuthorizationStateRequired
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrAuthorizationCodeRequired
	}

	attempt, err := service.findActiveAuthorizationAttempt(ctx, state)
	if err != nil {
		return nil, err
	}

	codeVerifier, err := service.credentialProtector.Decrypt(attempt.CodeVerifierCiphertext)
	if err != nil {
		return nil, fmt.Errorf("recovering PKCE verifier: %w", err)
	}
	credentials, err := service.oauthConnector.ExchangeAuthorizationCode(ctx, code, codeVerifier)
	if err != nil {
		if errors.Is(err, ErrAuthorizationCodeUnusable) ||
			errors.Is(err, ErrMarketplacePaymentsNotEnabled) {
			if consumeErr := service.authorizationAttemptRepository.Consume(ctx, attempt); consumeErr != nil {
				return nil, fmt.Errorf("consuming failed payment account authorization: %w", consumeErr)
			}
		}
		return nil, fmt.Errorf("exchanging payment account authorization code: %w", err)
	}
	if err := ValidateOAuthCredentials(credentials); err != nil {
		if consumeErr := service.authorizationAttemptRepository.Consume(ctx, attempt); consumeErr != nil {
			return nil, fmt.Errorf("consuming invalid payment account authorization: %w", consumeErr)
		}
		return nil, err
	}
	accessTokenCiphertext, err := service.credentialProtector.Encrypt(credentials.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("protecting payment account access token: %w", err)
	}
	var refreshTokenCiphertext []byte
	if credentials.RefreshToken != "" {
		refreshTokenCiphertext, err = service.credentialProtector.Encrypt(credentials.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("protecting payment account refresh token: %w", err)
		}
	}
	account, err := NewPaymentAccount(
		attempt.ProviderID,
		attempt.PaymentProvider,
		credentials.ExternalAccountID,
		accessTokenCiphertext,
		refreshTokenCiphertext,
		credentials.ExpiresOn,
		credentials.CanReceiveMarketplacePayments,
	)
	if err != nil {
		return nil, err
	}

	if err := service.paymentAccountRepository.SaveFromAuthorization(ctx, attempt.ID, account); err != nil {
		return nil, fmt.Errorf("completing payment account authorization: %w", err)
	}
	return account, nil
}

func (service *Service) RejectAuthorization(ctx context.Context, state string) error {
	state = strings.TrimSpace(state)
	if state == "" {
		return ErrAuthorizationStateRequired
	}
	attempt, err := service.findActiveAuthorizationAttempt(ctx, state)
	if err != nil {
		return err
	}
	if err := service.authorizationAttemptRepository.Consume(ctx, attempt); err != nil {
		return fmt.Errorf("consuming rejected payment account authorization: %w", err)
	}
	return nil
}

func (service *Service) findActiveAuthorizationAttempt(ctx context.Context, state string) (*AuthorizationAttempt, error) {
	attempt, err := service.authorizationAttemptRepository.FindByStateDigest(ctx, stateDigest(state))
	if err != nil {
		return nil, err
	}
	if attempt.IsExpired(service.clock.Now()) {
		return nil, ErrAuthorizationAttemptExpired
	}
	if attempt.PaymentProvider != service.oauthConnector.Provider() {
		return nil, ErrPaymentProviderMismatch
	}
	return attempt, nil
}

func (service *Service) GetConnection(ctx context.Context, authID string) (*PaymentAccount, error) {
	providerID, err := service.authenticatedProviderID(authID)
	if err != nil {
		return nil, err
	}

	account, err := service.paymentAccountRepository.FindByProviderID(ctx, providerID, service.oauthConnector.Provider())
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (service *Service) authenticatedProviderID(authID string) (int, error) {
	foundUser, err := service.userRepository.FindByAuthID(authID)
	if err != nil {
		return 0, err
	}
	foundProvider, ok := foundUser.(*provider.Provider)
	if !ok {
		return 0, ErrOnlyProvidersCanConnect
	}
	return foundProvider.ID(), nil
}

func stateDigest(state string) []byte {
	digest := sha256.Sum256([]byte(state))
	return digest[:]
}
