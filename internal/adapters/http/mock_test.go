package httpadapter

import (
	"context"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	operationreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/mock"
)

func buildTokenWithRole(subject, role string) (string, error) {
	token := jwt.New()
	for key, value := range map[string]any{
		jwt.SubjectKey:    subject,
		jwt.IssuerKey:     "test-issuer",
		jwt.AudienceKey:   []string{"test-audience"},
		"permissions":     []string{},
		"role":            role,
		jwt.ExpirationKey: time.Now().Add(time.Hour),
	} {
		if err := token.Set(key, value); err != nil {
			return "", err
		}
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256(), []byte("test-secret-do-not-use-in-production")))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

type collectionServiceMock struct{ mock.Mock }

func (service *collectionServiceMock) Summary(ctx context.Context, authID string, input provider.ActivityQueryInput) (*providerreadmodel.Collections, error) {
	arguments := service.Called(ctx, authID, input)
	if value := arguments.Get(0); value != nil {
		return value.(*providerreadmodel.Collections), arguments.Error(1)
	}
	return nil, arguments.Error(1)
}

func (service *collectionServiceMock) Detail(ctx context.Context, authID string, input provider.CollectionDetailInput) (*providerreadmodel.CollectionDetail, int, error) {
	arguments := service.Called(ctx, authID, input)
	if value := arguments.Get(0); value != nil {
		return value.(*providerreadmodel.CollectionDetail), arguments.Int(1), arguments.Error(2)
	}
	return nil, arguments.Int(1), arguments.Error(2)
}

type adminPaymentQueryServiceMock struct {
	mock.Mock
}

func (service *adminPaymentQueryServiceMock) Query(ctx context.Context, query payment.AdminPaymentQuery, subject, correlation string) (*readmodel.AdminPaymentPage, error) {
	arguments := service.Called(ctx, query, subject, correlation)
	if page, ok := arguments.Get(0).(*readmodel.AdminPaymentPage); ok {
		return page, arguments.Error(1)
	}
	return nil, arguments.Error(1)
}

type paymentUserFinderMock struct {
	mock.Mock
}

func (finder *paymentUserFinderMock) FindByAuthID(authID string) (user.User, error) {
	arguments := finder.Called(authID)
	if foundUser, ok := arguments.Get(0).(user.User); ok {
		return foundUser, arguments.Error(1)
	}
	return nil, arguments.Error(1)
}

type paymentIntentRepositoryMock struct {
	mock.Mock
}

func (repository *paymentIntentRepositoryMock) Save(ctx context.Context, intent *payment.Intent) error {
	return repository.Called(ctx, intent).Error(0)
}

func (repository *paymentIntentRepositoryMock) FindByID(ctx context.Context, id string) (*payment.Intent, error) {
	arguments := repository.Called(ctx, id)
	if intent, ok := arguments.Get(0).(*payment.Intent); ok {
		return intent, arguments.Error(1)
	}
	return nil, arguments.Error(1)
}

func (repository *paymentIntentRepositoryMock) FindLatestByProposalIDAndPurpose(ctx context.Context, proposalID int, purpose payment.Purpose) (*payment.Intent, error) {
	arguments := repository.Called(ctx, proposalID, purpose)
	if intent, ok := arguments.Get(0).(*payment.Intent); ok {
		return intent, arguments.Error(1)
	}
	return nil, arguments.Error(1)
}

type serviceProposalFinderMock struct {
	mock.Mock
}

func (finder *serviceProposalFinderMock) FindByID(ctx context.Context, id int) (*serviceproposal.ServiceProposal, error) {
	arguments := finder.Called(ctx, id)
	if proposal, ok := arguments.Get(0).(*serviceproposal.ServiceProposal); ok {
		return proposal, arguments.Error(1)
	}
	return nil, arguments.Error(1)
}

type operationFunnelQueryServiceMock struct {
	mock.Mock
}

func (service *operationFunnelQueryServiceMock) Query(ctx context.Context, query operation.FunnelQuery) (operationreadmodel.FunnelMetrics, error) {
	arguments := service.Called(ctx, query)
	if metrics, ok := arguments.Get(0).(operationreadmodel.FunnelMetrics); ok {
		return metrics, arguments.Error(1)
	}
	return operationreadmodel.FunnelMetrics{}, arguments.Error(1)
}

type reputationServiceMock struct{ mock.Mock }

func (service *reputationServiceMock) Query(ctx context.Context, authID string, input provider.ReputationQueryInput) (*providerreadmodel.Reputation, int, error) {
	arguments := service.Called(ctx, authID, input)
	if value := arguments.Get(0); value != nil {
		return value.(*providerreadmodel.Reputation), arguments.Int(1), arguments.Error(2)
	}
	return nil, arguments.Int(1), arguments.Error(2)
}

type conversionServiceMock struct{ mock.Mock }

func (service *conversionServiceMock) Query(ctx context.Context, authID string, input provider.ConversionQueryInput) (*providerreadmodel.Conversion, error) {
	arguments := service.Called(ctx, authID, input)
	if value := arguments.Get(0); value != nil {
		return value.(*providerreadmodel.Conversion), arguments.Error(1)
	}
	return nil, arguments.Error(1)
}
