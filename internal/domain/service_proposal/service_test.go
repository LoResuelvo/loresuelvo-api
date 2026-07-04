package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/assert"
)

var (
	validConsumerID               = 1
	validProviderAuth0ID          = "provider-auth0-id"
	validServiceDescription       = "Service description"
	validServiceAmount      int64 = 1000
	validServiceScheduledOn       = time.Now()
)

func TestCreateServiceProposal(t *testing.T) {
	providerRepo := new(MockProviderRepository)
	consumerRepo := new(MockConsumerRepository)

	providerRepo.
		On("FindByAuthID", validProviderAuth0ID).
		Return(&provider.Provider{}, nil)

	consumerRepo.
		On("FindByID", validConsumerID).
		Return(&consumer.Consumer{}, nil)

	service := serviceproposal.NewService(providerRepo, consumerRepo)

	serviceProposal, err := service.CreateServiceProposal(
		validProviderAuth0ID, validConsumerID, validServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	assert.NoError(t, err)
	assert.NotNil(t, serviceProposal)
}
