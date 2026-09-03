package bootstrap

import (
	"database/sql"
	"fmt"

	googlecalendar "github.com/LoResuelvo/loresuelvo-api/internal/adapters/google_calendar"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/calendar_connection_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/identity_verification_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_account_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_handler"
	didit "github.com/LoResuelvo/loresuelvo-api/internal/adapters/identityverification/didit"
	identityverificationfake "github.com/LoResuelvo/loresuelvo-api/internal/adapters/identityverification/fake"
	locationadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/location"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
)

type TestDoubles struct {
	CalendarEventPublisher  *googlecalendar.FakeEventPublisher
	ConsumerAddressResolver *locationadapter.FakeAddressResolver
	IdentityVerifier        *identityverificationfake.Verifier
}

func newTestIdentityVerificationWebhook() identity_verification_handler.IdentityVerificationWebhook {
	webhook, err := didit.NewWebhookAdapter("test-didit-webhook-secret")
	if err != nil {
		panic(fmt.Errorf("configuring test identity verification webhook: %w", err))
	}
	return webhook
}

func NewTestDependencies(
	database *sql.DB,
	chatbot conversation.Chatbot,
	paymentAccountOAuthConnector paymentaccount.OAuthConnector,
	paymentGateway payment.Gateway,
	webhookVerifier payment_handler.WebhookVerifier,
	credentialProtector paymentaccount.CredentialProtector,
	secretGenerator paymentaccount.SecretGenerator,
	paymentAccountHandlerConfig payment_account_handler.Config,
) (*Dependencies, TestDoubles, error) {
	calendarEventPublisher := googlecalendar.NewFakeEventPublisher()
	consumerAddressResolver := locationadapter.NewFakeAddressResolver()
	identityVerifier := identityverificationfake.NewVerifier()
	dependencies, err := newDependencies(database, dependencyAdapters{
		chatbot:                      chatbot,
		paymentAccountOAuthConnector: paymentAccountOAuthConnector,
		paymentGateway:               paymentGateway,
		webhookVerifier:              webhookVerifier,
		credentialProtector:          credentialProtector,
		secretGenerator:              secretGenerator,
		paymentAccountHandlerConfig:  paymentAccountHandlerConfig,
		calendarOAuthConnector:       googlecalendar.NewFakeOAuthClient(),
		calendarCredentialProtector:  credentialProtector,
		calendarEventPublisher:       calendarEventPublisher,
		calendarHandlerConfig: calendar_connection_handler.Config{
			ConnectionSuccessURL:   "/me",
			ConnectionCancelledURL: "/me",
		},
		identityVerifier:        identityVerifier,
		addressResolverOverride: consumerAddressResolver,
		recommendationConfig:    conversation.DefaultProviderRecommendationConfig(),
		identityWebhook:         newTestIdentityVerificationWebhook(),
	})
	if err != nil {
		return nil, TestDoubles{}, err
	}
	return dependencies, TestDoubles{
		CalendarEventPublisher:  calendarEventPublisher,
		ConsumerAddressResolver: consumerAddressResolver,
		IdentityVerifier:        identityVerifier,
	}, nil
}
