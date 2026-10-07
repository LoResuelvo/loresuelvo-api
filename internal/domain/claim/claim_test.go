package claim

import (
	"strings"
	"testing"
	"time"

	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
)

func TestSubmissionNormalizesUnicodeAndFileOrder(t *testing.T) {
	ref, err := NewReference(ReferenceKindJobRequest, "00012")
	require.NoError(t, err)
	input := Submission{Reference: ref, Reason: ReasonNoncompliance, Description: "\u2003hecho\u2003", ImageFileIDs: []string{"550E8400-E29B-41D4-A716-446655440001", "550e8400-e29b-41d4-a716-446655440000"}}
	normalized, err := input.Normalize()
	require.NoError(t, err)
	require.Equal(t, "12", normalized.Reference.ID())
	require.Equal(t, "hecho", normalized.Description)
	other := input
	other.Description = "hecho"
	other.ImageFileIDs = []string{"550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001"}
	other, err = other.Normalize()
	require.NoError(t, err)
	require.Equal(t, normalized.Fingerprint(), other.Fingerprint())
}
func TestSubmissionRejectsInvalidValues(t *testing.T) {
	ref, err := NewReference(ReferenceKindJobRequest, "1")
	require.NoError(t, err)
	for _, description := range []string{"", "\u2003\n", strings.Repeat("é", 5001), "test\x00imony"} {
		t.Run(description[:min(len(description), 10)], func(t *testing.T) {
			_, err := (Submission{Reference: ref, Reason: ReasonDamage, Description: description}).Normalize()
			require.ErrorIs(t, err, ErrInvalidSubmission)
		})
	}
	_, err = (Submission{Reference: ref, Reason: ReasonDamage, Description: strings.Repeat("é", 5000)}).Normalize()
	require.NoError(t, err)
	for _, id := range []string{"0", "-1", "2147483648", "abc", " 1"} {
		_, err := NewReference(ReferenceKindJobRequest, id)
		require.ErrorIs(t, err, ErrInvalidSubmission)
	}
	_, err = (Submission{Reference: ref, Reason: Reason("unknown"), Description: "valid"}).Normalize()
	require.ErrorIs(t, err, ErrInvalidSubmission)
	_, err = (Submission{Reference: ref, Reason: ReasonDamage, Description: "valid", ImageFileIDs: []string{"550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"}}).Normalize()
	require.ErrorIs(t, err, ErrInvalidSubmission)
}
func TestNewClaimOwnsInitialAction(t *testing.T) {
	ref, err := NewReference(ReferenceKindJobRequest, "1")
	require.NoError(t, err)
	input, err := (Submission{Reference: ref, Reason: ReasonDamage, Description: "valid"}).Normalize()
	require.NoError(t, err)
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	found, err := New(Claimant{ID: 1, Party: PartyProvider}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: 1}, "550e8400-e29b-41d4-a716-446655440000", input, now)
	require.NoError(t, err)
	require.Equal(t, StatusOpen, found.Status)
	require.Len(t, found.Actions, 1)
	require.Equal(t, PartyProvider, found.Actions[0].ActorParty)
	require.Nil(t, found.Resolution)
	require.NotNil(t, found.ImageFileIDs)
}

func TestClaimCreationUsesPersistableTimestampPrecision(t *testing.T) {
	ref, err := NewReference(ReferenceKindJobRequest, "1")
	require.NoError(t, err)
	now := time.Date(2026, 10, 7, 0, 0, 0, 123456789, time.FixedZone("offset", 3600))
	found, err := New(Claimant{ID: 1, Party: PartyConsumer}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: 1}, "550e8400-e29b-41d4-a716-446655440000", Submission{Reference: ref, Reason: ReasonDamage, Description: "valid"}, now)
	require.NoError(t, err)
	require.Equal(t, now.UTC().Truncate(time.Microsecond), found.CreatedOn)
	require.Equal(t, found.CreatedOn, found.Actions[0].CreatedOn)
}
