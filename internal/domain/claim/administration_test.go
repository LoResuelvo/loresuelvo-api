package claim

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStartReviewPreservesSubmission(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	c := &Claim{ID: 1, Status: StatusOpen, CreatedOn: now.Add(-time.Hour), Description: "original"}
	action, err := c.StartReview(2, now)
	require.NoError(t, err)
	require.Equal(t, StatusInReview, c.Status)
	require.Equal(t, PartyOperator, action.ActorParty)
	require.Equal(t, "original", c.Description)
	_, err = c.StartReview(2, now)
	require.ErrorIs(t, err, ErrInvalidTransition)
}

func TestResolveMapsFinalStateAndNormalizesReasoning(t *testing.T) {
	for _, kind := range []ResolutionType{ResolutionTypeConsumerFavor, ResolutionTypeProviderFavor, ResolutionTypeAgreement, ResolutionTypeWithoutMerit} {
		t.Run(string(kind), func(t *testing.T) {
			now := time.Now().UTC()
			review := now.Add(-time.Hour)
			c := &Claim{ID: 1, Status: StatusInReview, ReviewStartedOn: &review}
			amount := int64(1500)
			_, err := c.Resolve(2, ResolutionInput{Type: kind, Reasoning: "  ñ válida  ", SuggestedAmountMinor: &amount}, now)
			require.NoError(t, err)
			require.Equal(t, "ñ válida", c.Resolution.Reasoning)
			require.Equal(t, "ARS", c.Resolution.SuggestedCompensation.Currency)
			expected := StatusResolved
			if kind == ResolutionTypeWithoutMerit {
				expected = StatusDismissed
			}
			require.Equal(t, expected, c.Status)
		})
	}
}

func TestResolutionRejectsInvalidReasoningAndAmounts(t *testing.T) {
	for _, text := range []string{"", "  ", strings.Repeat("ñ", 5001), "x\x00y", string([]byte{0xff})} {
		_, err := (ResolutionInput{Type: ResolutionTypeAgreement, Reasoning: text}).Normalize()
		require.ErrorIs(t, err, ErrInvalidResolution)
	}
	_, err := (ResolutionInput{Type: ResolutionTypeAgreement, Reasoning: strings.Repeat("ñ", 5000)}).Normalize()
	require.NoError(t, err)
	for _, amount := range []int64{0, -1} {
		_, err := (ResolutionInput{Type: ResolutionTypeAgreement, Reasoning: "valid", SuggestedAmountMinor: &amount}).Normalize()
		require.ErrorIs(t, err, ErrInvalidResolution)
	}
}
