package calendarconnection

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

const authorizationAttemptLifetime = 10 * time.Minute

type Service struct {
	userRepository                 UserRepository
	authorizationAttemptRepository AuthorizationAttemptRepository
	connectionRepository           ConnectionRepository
	oauthConnector                 OAuthConnector
	credentialProtector            CredentialProtector
	secretGenerator                SecretGenerator
	clock                          Clock
}

func NewService(
	userRepository UserRepository,
	authorizationAttemptRepository AuthorizationAttemptRepository,
	connectionRepository ConnectionRepository,
	oauthConnector OAuthConnector,
	credentialProtector CredentialProtector,
	secretGenerator SecretGenerator,
	clock Clock,
) *Service {
	return &Service{
		userRepository:                 userRepository,
		authorizationAttemptRepository: authorizationAttemptRepository,
		connectionRepository:           connectionRepository,
		oauthConnector:                 oauthConnector,
		credentialProtector:            credentialProtector,
		secretGenerator:                secretGenerator,
		clock:                          clock,
	}
}

func (service *Service) StartAuthorization(ctx context.Context, authID string) (*Authorization, error) {
	foundUser, err := service.userRepository.FindByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if foundUser == nil || foundUser.ID() <= 0 {
		return nil, ErrUserNotFound
	}

	state, err := service.secretGenerator.Generate()
	if err != nil {
		return nil, fmt.Errorf("generating calendar authorization state: %w", err)
	}
	codeVerifier, err := service.secretGenerator.Generate()
	if err != nil {
		return nil, fmt.Errorf("generating calendar PKCE verifier: %w", err)
	}
	codeVerifierCiphertext, err := service.credentialProtector.Encrypt(codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("protecting calendar PKCE verifier: %w", err)
	}
	authorizationURL, err := service.oauthConnector.AuthorizationURL(state, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("building calendar authorization URL: %w", err)
	}
	attempt, err := NewAuthorizationAttempt(
		foundUser.ID(),
		stateDigest(state),
		codeVerifierCiphertext,
		service.clock.Now().Add(authorizationAttemptLifetime),
	)
	if err != nil {
		return nil, err
	}
	if err := service.authorizationAttemptRepository.Save(ctx, attempt); err != nil {
		return nil, fmt.Errorf("saving calendar authorization attempt: %w", err)
	}

	return &Authorization{URL: authorizationURL, State: state}, nil
}

func (service *Service) CompleteAuthorization(ctx context.Context, state, code string) (*Connection, error) {
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
		return nil, fmt.Errorf("recovering calendar PKCE verifier: %w", err)
	}
	credentials, err := service.oauthConnector.ExchangeAuthorizationCode(ctx, code, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("exchanging calendar authorization code: %w", err)
	}
	if err := ValidateAuthorizationCredentials(credentials); err != nil {
		if consumeErr := service.authorizationAttemptRepository.Consume(ctx, attempt); consumeErr != nil {
			return nil, fmt.Errorf("consuming invalid calendar authorization: %w", consumeErr)
		}
		return nil, err
	}
	refreshTokenCiphertext, err := service.credentialProtector.Encrypt(credentials.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("protecting calendar refresh token: %w", err)
	}
	connection, err := NewConnection(attempt.UserID, credentials.CalendarID, refreshTokenCiphertext, service.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := service.connectionRepository.SaveFromAuthorization(ctx, attempt.ID, connection); err != nil {
		return nil, fmt.Errorf("completing calendar authorization: %w", err)
	}
	return connection, nil
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
		return fmt.Errorf("consuming rejected calendar authorization: %w", err)
	}
	return nil
}

func (service *Service) GetConnectionStatus(ctx context.Context, authID string) (string, error) {
	foundUser, err := service.userRepository.FindByAuthID(authID)
	if err != nil {
		return "", err
	}
	if foundUser == nil || foundUser.ID() <= 0 {
		return "", ErrUserNotFound
	}
	connection, err := service.connectionRepository.FindByUserID(ctx, foundUser.ID())
	if errors.Is(err, ErrConnectionNotFound) {
		return StatusDisconnected, nil
	}
	if err != nil {
		return "", fmt.Errorf("finding calendar connection: %w", err)
	}
	return connection.Status(), nil
}

func (service *Service) GetConnection(ctx context.Context, authID string) (*Connection, error) {
	foundUser, err := service.userRepository.FindByAuthID(authID)
	if err != nil {
		return nil, err
	}
	if foundUser == nil || foundUser.ID() <= 0 {
		return nil, ErrUserNotFound
	}
	connection, err := service.connectionRepository.FindByUserID(ctx, foundUser.ID())
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func (service *Service) findActiveAuthorizationAttempt(ctx context.Context, state string) (*AuthorizationAttempt, error) {
	attempt, err := service.authorizationAttemptRepository.FindByStateDigest(ctx, stateDigest(state))
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, ErrAuthorizationAttemptNotFound
	}
	if attempt.IsConsumed() {
		return nil, ErrAuthorizationAttemptConsumed
	}
	if attempt.IsExpired(service.clock.Now()) {
		return nil, ErrAuthorizationAttemptExpired
	}
	return attempt, nil
}

func stateDigest(state string) []byte {
	digest := sha256.Sum256([]byte(state))
	return digest[:]
}
