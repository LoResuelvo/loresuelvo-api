package claim

import (
	"errors"
	"testing"
	"time"

	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testKey = "550e8400-e29b-41d4-a716-446655440000"
const testFile = "550e8400-e29b-41d4-a716-446655440001"

func TestSubmitRecoversCurrentClaimBeforeOperationAndEvidenceValidation(t *testing.T) {
	for _, status := range []Status{StatusOpen, StatusInReview, StatusResolved, StatusDismissed} {
		t.Run(string(status), func(t *testing.T) {
			ref, err := NewReference(ReferenceKindJobRequest, "1")
			require.NoError(t, err)
			input, err := (Submission{Reference: ref, Reason: ReasonDamage, Description: "original", ImageFileIDs: []string{testFile}}).Normalize()
			require.NoError(t, err)
			existing := Rehydrate(Claim{ID: 7, Status: status}, testKey, input.Fingerprint())
			users := &userFinderMock{}
			repo := &repositoryMock{}
			users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyConsumer}, nil).Once()
			repo.On("FindBySubmissionKey", mock.Anything, 1, testKey).Return(existing, nil).Once()
			result, err := NewService(repo, users, nil, nil, nil).Submit(t.Context(), "auth", testKey, input)
			require.NoError(t, err)
			require.False(t, result.Created)
			require.Same(t, existing, result.Claim)
			users.AssertExpectations(t)
			repo.AssertExpectations(t)
		})
	}
}
func TestSubmitReloadsKeyBeforeReportingEveryConstraintConflict(t *testing.T) {
	for _, conflict := range []error{ErrPersistenceConflict, ErrOpenClaimConflict, ErrInvalidEvidence} {
		t.Run(conflict.Error(), func(t *testing.T) {
			ref, err := NewReference(ReferenceKindJobRequest, "1")
			require.NoError(t, err)
			input, err := (Submission{Reference: ref, Reason: ReasonDamage, Description: "original"}).Normalize()
			require.NoError(t, err)
			existing := Rehydrate(Claim{ID: 7, Status: StatusOpen}, testKey, input.Fingerprint())
			who := Claimant{ID: 1, Party: PartyConsumer}
			op := &operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: 1}
			users := &userFinderMock{}
			repo := &repositoryMock{}
			operations := &operationReferenceResolverMock{}
			clock := &clockMock{}
			users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&who, nil).Once()
			repo.On("FindBySubmissionKey", mock.Anything, 1, testKey).Return(nil, nil).Once()
			repo.On("FindBySubmissionKey", mock.Anything, 1, testKey).Return(existing, nil).Once()
			operations.On("ResolveClaimOperationReference", mock.Anything, who, ref).Return(op, nil).Once()
			clock.On("Now").Return(time.Now()).Once()
			repo.On("Save", mock.Anything, mock.Anything).Return(conflict).Once()
			result, err := NewService(repo, users, operations, nil, clock).Submit(t.Context(), "auth", testKey, input)
			require.NoError(t, err)
			require.Equal(t, 7, result.Claim.ID)
			require.False(t, result.Created)
			repo.AssertExpectations(t)
			operations.AssertExpectations(t)
			clock.AssertExpectations(t)
		})
	}
}
func TestSubmitDoesNotMaskReplayLookupFailure(t *testing.T) {
	ref, err := NewReference(ReferenceKindJobRequest, "1")
	require.NoError(t, err)
	failure := errors.New("db unavailable")
	users := &userFinderMock{}
	repo := &repositoryMock{}
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyProvider}, nil).Once()
	repo.On("FindBySubmissionKey", mock.Anything, 1, testKey).Return(nil, failure).Once()
	result, err := NewService(repo, users, nil, nil, nil).Submit(t.Context(), "auth", testKey, Submission{Reference: ref, Reason: ReasonDamage, Description: "original"})
	require.Nil(t, result)
	require.ErrorIs(t, err, failure)
	repo.AssertExpectations(t)
}
func TestGetDoesNotResolveEvidenceBeforeOwnership(t *testing.T) {
	users, repo, images := new(userFinderMock), new(repositoryMock), new(evidenceImagesMock)
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyProvider}, nil).Once()
	repo.On("FindOwnedByID", mock.Anything, 1, 7).Return((*Claim)(nil), nil).Once()
	result, err := NewService(repo, users, nil, images, nil).Get(t.Context(), "auth", 7)
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrNotFound)
	images.AssertNotCalled(t, "ResolveClaimEvidenceImage", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
	users.AssertExpectations(t)
}

func TestGetDoesNotResolveUnlinkedOwnedEvidence(t *testing.T) {
	users, repo, images := new(userFinderMock), new(repositoryMock), new(evidenceImagesMock)
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyConsumer}, nil).Once()
	repo.On("FindOwnedByID", mock.Anything, 1, 7).Return(&Claim{ID: 7}, nil).Once()
	result, err := NewService(repo, users, nil, images, nil).Get(t.Context(), "auth", 7)
	require.NoError(t, err)
	require.Empty(t, result.Images)
	images.AssertNotCalled(t, "ResolveClaimEvidenceImage", mock.Anything, mock.Anything, testFile)
	repo.AssertExpectations(t)
	users.AssertExpectations(t)
}
func TestListCriteriaDefaultsAndLimits(t *testing.T) {
	normalized, err := (ListCriteria{}).Normalize()
	require.NoError(t, err)
	require.Equal(t, 1, normalized.Page)
	require.Equal(t, 20, normalized.Limit)
	invalid := Status("invalid")
	for _, criteria := range []ListCriteria{{Page: -1}, {Limit: -1}, {Limit: 101}, {Page: 2147483648}, {Status: &invalid}} {
		_, err := criteria.Normalize()
		require.ErrorIs(t, err, ErrInvalidCriteria)
	}
}

func TestSubmitValidatesEvidenceAndPersistsOneAggregate(t *testing.T) {
	ref, err := NewReference(ReferenceKindJobRequest, "1")
	require.NoError(t, err)
	input := Submission{Reference: ref, Reason: ReasonDamage, Description: "original", ImageFileIDs: []string{testFile}}
	who := Claimant{ID: 1, Party: PartyConsumer}
	op := &operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: 1}
	users := &userFinderMock{}
	repo := &repositoryMock{}
	operations := &operationReferenceResolverMock{}
	images := &evidenceImagesMock{}
	clock := &clockMock{}
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&who, nil).Once()
	repo.On("FindBySubmissionKey", mock.Anything, 1, testKey).Return(nil, nil).Once()
	operations.On("ResolveClaimOperationReference", mock.Anything, who, ref).Return(op, nil).Once()
	images.On("ValidateClaimEvidenceImages", mock.Anything, "auth", []string{testFile}).Return(nil).Once()
	clock.On("Now").Return(time.Now()).Once()
	repo.On("Save", mock.Anything, mock.MatchedBy(func(c *Claim) bool {
		return len(c.Actions) == 1 && c.Description == "original" && len(c.ImageFileIDs) == 1
	})).Run(func(a mock.Arguments) { a.Get(1).(*Claim).ID = 7 }).Return(nil).Once()
	result, err := NewService(repo, users, operations, images, clock).Submit(t.Context(), "auth", testKey, input)
	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, 7, result.Claim.ID)
	repo.AssertExpectations(t)
	images.AssertExpectations(t)
	images.AssertNotCalled(t, "ResolveClaimEvidenceImage", mock.Anything, mock.Anything, mock.Anything)
}
func TestGetFailsClosedWhenAnEvidenceURLCannotBeResolved(t *testing.T) {
	failure := errors.New("storage unavailable")
	secondFile := "550e8400-e29b-41d4-a716-446655440002"
	users, repo, images := new(userFinderMock), new(repositoryMock), new(evidenceImagesMock)
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyConsumer}, nil).Once()
	repo.On("FindOwnedByID", mock.Anything, 1, 7).Return(&Claim{ID: 7, ImageFileIDs: []string{testFile, secondFile}}, nil).Once()
	images.On("ResolveClaimEvidenceImage", mock.Anything, "auth", testFile).Return("https://private/first?signature=temporary", nil).Once()
	images.On("ResolveClaimEvidenceImage", mock.Anything, "auth", secondFile).Return("", failure).Once()
	result, err := NewService(repo, users, nil, images, nil).Get(t.Context(), "auth", 7)
	require.Nil(t, result)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, err, ErrEvidenceAccessUnavailable)
	images.AssertExpectations(t)
	users.AssertExpectations(t)
	repo.AssertExpectations(t)
}
func TestSubmissionEnforcesExactReasonAndThreeImages(t *testing.T) {
	ref, err := NewReference(ReferenceKindJobRequest, "1")
	require.NoError(t, err)
	require.Equal(t, Reason("non_compliance"), ReasonNoncompliance)
	_, err = (Submission{Reference: ref, Reason: Reason("noncompliance"), Description: "original"}).Normalize()
	require.ErrorIs(t, err, ErrInvalidSubmission)
	files := []string{testKey, testFile, "550e8400-e29b-41d4-a716-446655440002", "550e8400-e29b-41d4-a716-446655440003"}
	_, err = (Submission{Reference: ref, Reason: ReasonDamage, Description: "original", ImageFileIDs: files[:3]}).Normalize()
	require.NoError(t, err)
	_, err = (Submission{Reference: ref, Reason: ReasonDamage, Description: "original", ImageFileIDs: files}).Normalize()
	require.ErrorIs(t, err, ErrInvalidSubmission)
}

func TestGetRejectsOutOfRangeIDBeforePersistence(t *testing.T) {
	users := new(userFinderMock)
	repo := new(repositoryMock)
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyConsumer}, nil).Once()
	svc := NewService(repo, users, nil, nil, nil)
	_, err := svc.Get(t.Context(), "auth", 2147483648)
	require.ErrorIs(t, err, ErrNotFound)
	repo.AssertNotCalled(t, "FindOwnedByID", mock.Anything, mock.Anything, mock.Anything)
	users.AssertExpectations(t)
}

func TestGetResolvesOnlyLinkedEvidenceAfterOwnership(t *testing.T) {
	users, repo, images := new(userFinderMock), new(repositoryMock), new(evidenceImagesMock)
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyConsumer}, nil).Once()
	repo.On("FindOwnedByID", mock.Anything, 1, 7).Return(&Claim{ID: 7, ImageFileIDs: []string{testFile}}, nil).Once()
	images.On("ResolveClaimEvidenceImage", mock.Anything, "auth", testFile).Return("https://private/image?signature=temporary", nil).Once()
	result, err := NewService(repo, users, nil, images, nil).Get(t.Context(), "auth", 7)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, testFile, result.Images[0].FileID)
	require.Equal(t, "https://private/image?signature=temporary", result.Images[0].URL)
	images.AssertExpectations(t)
	users.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestListDoesNotResolveEvidenceURLs(t *testing.T) {
	users, repo, images := new(userFinderMock), new(repositoryMock), new(evidenceImagesMock)
	users.On("FindClaimantByAuthID", mock.Anything, "auth").Return(&Claimant{ID: 1, Party: PartyConsumer}, nil).Once()
	repo.On("FindOwnedPage", mock.Anything, 1, ListCriteria{Page: 1, Limit: 20}).Return(&Page{}, nil).Once()
	_, err := NewService(repo, users, nil, images, nil).List(t.Context(), "auth", ListCriteria{})
	require.NoError(t, err)
	require.Empty(t, images.Calls)
	users.AssertExpectations(t)
	repo.AssertExpectations(t)
}
