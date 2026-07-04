package serviceproposal_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/assert"
)

var (
	invalidServiceAmount int64 = -100
	validProvider              = &provider.Provider{}
	validConsumer              = &consumer.Consumer{}
)

func TestInvalidAmount(t *testing.T) {
	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, invalidServiceAmount,
		validServiceScheduledOn, validServiceDescription)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}
