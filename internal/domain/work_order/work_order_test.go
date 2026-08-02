package workorder_test

import (
	"testing"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkOrderCompletesPaymentWithConsumerConfirmationAuthorization(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	issuedOn := time.Date(2026, time.July, 6, 13, 5, 0, 0, time.UTC)
	order := workOrderFixture(84, 10, 20, acceptedOn.Add(48*time.Hour))
	authorization, err := workorder.NewCompletionAuthorization([]byte("encrypted-code"), issuedOn)
	require.NoError(t, err)

	err = order.CompletePayment(authorization)

	require.NoError(t, err)
	assert.Equal(t, workorder.StatusPaid, order.Status)
	assert.Same(t, authorization, order.CompletionAuthorization)
	assert.Equal(t, issuedOn, authorization.IssuedOn())
	assert.Equal(t, []byte("encrypted-code"), authorization.CodeCiphertext())

	consumerAuthorization, err := order.ConfirmationAuthorizationFor(order.ConsumerID())
	require.NoError(t, err)
	assert.Same(t, authorization, consumerAuthorization)
	_, err = order.ConfirmationAuthorizationFor(order.ConsumerID() + 1)
	assert.ErrorIs(t, err, workorder.ErrOnlyConsumerCanViewConfirmationCode)
}

func TestScheduledWorkOrderHasNoConfirmationAuthorization(t *testing.T) {
	order := workOrderFixture(84, 10, 20, time.Now().UTC())

	authorization, err := order.ConfirmationAuthorizationFor(order.ConsumerID())

	assert.Nil(t, authorization)
	assert.ErrorIs(t, err, workorder.ErrConfirmationCodeNotAvailable)
}

func TestWorkOrderRejectsInvalidCompletionAuthorization(t *testing.T) {
	order := workOrderFixture(84, 10, 20, time.Now().UTC())

	err := order.CompletePayment(&workorder.CompletionAuthorization{})

	assert.ErrorIs(t, err, workorder.ErrInvalidCompletionAuthorization)
	assert.Equal(t, workorder.StatusScheduled, order.Status)
	assert.Nil(t, order.CompletionAuthorization)
}

func TestConfirmationCodeRequiresExactlyFourDigits(t *testing.T) {
	for _, invalid := range []string{"", "123", "12345", "12a4", "１２３４"} {
		code, err := workorder.NewConfirmationCode(invalid)
		assert.Empty(t, code)
		assert.ErrorIs(t, err, workorder.ErrInvalidCompletionAuthorization)
	}
	code, err := workorder.NewConfirmationCode("0042")
	require.NoError(t, err)
	assert.Equal(t, "0042", code.String())
}

func TestNewWorkOrderStartsScheduled(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)

	proposal := serviceproposal.ServiceProposal{
		ID: 1,
	}

	order, err := workorder.New(&proposal, acceptedOn)

	assert.NoError(t, err)
	assert.Equal(t, &proposal, order.ServiceProposal)
	assert.Equal(t, workorder.StatusScheduled, order.Status)
	assert.Equal(t, acceptedOn, order.AcceptedOn)
}
