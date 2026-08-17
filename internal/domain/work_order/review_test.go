package workorder_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReviewNormalizesDescription(t *testing.T) {
	review, err := workorder.NewReview(5, "  Trabajo prolijo y excelente atención.  ")

	require.NoError(t, err)
	assert.Equal(t, 5, review.Rating())
	assert.Equal(t, "Trabajo prolijo y excelente atención.", review.Description())

	emptyReview, err := workorder.NewReview(4, " \t\n ")

	require.NoError(t, err)
	assert.Equal(t, "", emptyReview.Description())
}

func TestNewReviewAcceptsRatingBoundaries(t *testing.T) {
	for _, rating := range []int{1, 5} {
		t.Run("rating "+strconv.Itoa(rating), func(t *testing.T) {
			review, err := workorder.NewReview(rating, "Trabajo correcto")

			require.NoError(t, err)
			assert.Equal(t, rating, review.Rating())
		})
	}
}

func TestNewReviewRejectsRatingOutsideAllowedRange(t *testing.T) {
	for _, rating := range []int{0, 6} {
		t.Run("invalid rating", func(t *testing.T) {
			_, err := workorder.NewReview(rating, "Trabajo correcto")

			assert.ErrorIs(t, err, workorder.ErrReviewRatingOutOfRange)
		})
	}
}

func TestNewReviewRejectsDescriptionLongerThanFiveHundredCharacters(t *testing.T) {
	_, err := workorder.NewReview(5, strings.Repeat("á", 501))

	assert.ErrorIs(t, err, workorder.ErrReviewDescriptionTooLong)
}

func TestNewReviewAcceptsDescriptionWithFiveHundredCharacters(t *testing.T) {
	description := strings.Repeat("á", 500)

	review, err := workorder.NewReview(5, description)

	require.NoError(t, err)
	assert.Equal(t, description, review.Description())
}

func TestPaidWorkOrderAcceptsReviewFromItsConsumer(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	review, err := workorder.NewReview(5, "Trabajo prolijo")
	require.NoError(t, err)

	err = order.AddReview(reviewer, review)

	require.NoError(t, err)
	assert.Equal(t, workorder.StatusPaid, order.Status())
	assert.Same(t, review, order.Review())
}

func TestWorkOrderRejectsReviewFromAnotherConsumer(t *testing.T) {
	order, _ := paidWorkOrderForReview(t)
	otherConsumer := &consumer.Consumer{
		BaseUser: user.RehydrateBaseUser(99, "auth0|other", "other@example.com", "Other", "Consumer", consumer.Role, nil),
	}
	review, err := workorder.NewReview(5, "Trabajo correcto")
	require.NoError(t, err)

	err = order.AddReview(otherConsumer, review)

	assert.ErrorIs(t, err, workorder.ErrOnlyWorkOrderConsumerCanReview)
	assert.Nil(t, order.Review())
}

func TestWorkOrderRejectsReviewWithoutConsumer(t *testing.T) {
	order, _ := paidWorkOrderForReview(t)
	review, err := workorder.NewReview(5, "Trabajo correcto")
	require.NoError(t, err)

	err = order.AddReview(nil, review)

	assert.ErrorIs(t, err, workorder.ErrOnlyWorkOrderConsumerCanReview)
	assert.Nil(t, order.Review())
}

func TestUnpaidWorkOrderRejectsReview(t *testing.T) {
	for _, status := range []workorder.Status{workorder.StatusScheduled, workorder.StatusAwaitingPayment} {
		t.Run(string(status), func(t *testing.T) {
			order, reviewer := workOrderWithStatusForReview(t, status)
			review, err := workorder.NewReview(5, "Trabajo correcto")
			require.NoError(t, err)

			err = order.AddReview(reviewer, review)

			assert.ErrorIs(t, err, workorder.ErrWorkOrderNotPaid)
			assert.Equal(t, status, order.Status())
			assert.Nil(t, order.Review())
		})
	}
}

func TestWorkOrderRejectsMissingReview(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)

	err := order.AddReview(reviewer, nil)

	assert.ErrorIs(t, err, workorder.ErrReviewRequired)
	assert.Nil(t, order.Review())
}

func TestWorkOrderRejectsDuplicateReview(t *testing.T) {
	order, reviewer := paidWorkOrderForReview(t)
	firstReview, err := workorder.NewReview(5, "Trabajo prolijo")
	require.NoError(t, err)
	secondReview, err := workorder.NewReview(3, "Otra opinión")
	require.NoError(t, err)
	require.NoError(t, order.AddReview(reviewer, firstReview))

	err = order.AddReview(reviewer, secondReview)

	assert.ErrorIs(t, err, workorder.ErrReviewAlreadyExists)
	assert.Same(t, firstReview, order.Review())
}

func paidWorkOrderForReview(t *testing.T) (*workorder.WorkOrder, *consumer.Consumer) {
	t.Helper()

	scheduledOn := time.Date(2026, time.August, 15, 15, 0, 0, 0, time.UTC)
	order := workOrderFixture(84, 10, 20, scheduledOn)
	report, err := workorder.NewCompletionReport("Trabajo terminado", []string{"file-1"}, scheduledOn)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(order.ProviderID(), report))
	require.NoError(t, order.RegisterApprovedBalancePayment(scheduledOn.Add(time.Hour)))

	reviewer := &consumer.Consumer{
		BaseUser: user.RehydrateBaseUser(order.ConsumerID(), "auth0|consumer", "consumer@example.com", "Ana", "Consumer", consumer.Role, nil),
	}
	return order, reviewer
}

func workOrderWithStatusForReview(t *testing.T, status workorder.Status) (*workorder.WorkOrder, *consumer.Consumer) {
	t.Helper()

	order := workOrderFixture(84, 10, 20, time.Date(2026, time.August, 15, 15, 0, 0, 0, time.UTC))
	if status == workorder.StatusAwaitingPayment {
		report, err := workorder.NewCompletionReport("Trabajo terminado", []string{"file-1"}, order.ScheduledOn())
		require.NoError(t, err)
		require.NoError(t, order.ReportCompletion(order.ProviderID(), report))
	}
	reviewer := &consumer.Consumer{
		BaseUser: user.RehydrateBaseUser(order.ConsumerID(), "auth0|consumer", "consumer@example.com", "Ana", "Consumer", consumer.Role, nil),
	}
	return order, reviewer
}
