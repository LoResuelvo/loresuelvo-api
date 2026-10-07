package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newStoredClaim(t *testing.T, claimantID, requestID int, party claim.Party, key string) *claim.Claim {
	t.Helper()
	reference, err := claim.NewReference(claim.ReferenceKindJobRequest, strconv.Itoa(requestID))
	require.NoError(t, err)
	found, err := claim.New(claim.Claimant{ID: claimantID, Party: party}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: requestID}, key, claim.Submission{Reference: reference, Reason: claim.ReasonDamage, Description: "original testimony"}, time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return found
}
func TestClaimRepositoryHydratesImagesInitialActionAndResolution(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, time.Now(), "pending")
	repository := repositories.NewClaimRepository(fixture.testContext.database)
	found := newStoredClaim(t, consumerID, request.ID, claim.PartyConsumer, uuid.NewString())
	image := uuid.NewString()
	_, err := fixture.testContext.database.Exec(`INSERT INTO files(id,key,bucket,original_name,mime_type,size_bytes,status,visibility,purpose,uploaded_by_auth_id,created_on,updated_on) VALUES($1::uuid,$1::text,'private','image.jpg','image/jpeg',100,'confirmed','private','claim_evidence_image','auth0|job-request-consumer',NOW(),NOW())`, image)
	require.NoError(t, err)
	found.ImageFileIDs = []string{image}
	review := found.CreatedOn.Add(time.Hour)
	closed := review.Add(time.Hour)
	found.Status = claim.StatusResolved
	found.ReviewStartedOn = &review
	found.ClosedOn = &closed
	found.Resolution = &claim.Resolution{Type: claim.ResolutionTypeConsumerFavor, Reasoning: "Formal reasoning", ResolvedOn: closed, SuggestedCompensation: &claim.SuggestedCompensation{AmountMinor: 1500, Currency: "ARS", Unit: "minor"}}
	require.NoError(t, repository.Save(t.Context(), found))
	hydrated, err := repository.FindOwnedByID(t.Context(), consumerID, found.ID)
	require.NoError(t, err)
	require.Equal(t, found.ImageFileIDs, hydrated.ImageFileIDs)
	require.Equal(t, found.Actions, hydrated.Actions)
	require.Equal(t, found.Resolution, hydrated.Resolution)
	require.Equal(t, found.SubmissionFingerprint(), hydrated.SubmissionFingerprint())
}
func TestClaimRepositoryRollbackDoesNotConsumeSubmissionKey(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	request := fixture.jobRequest(t, consumerID, providerID, time.Now(), "pending")
	repository := repositories.NewClaimRepository(fixture.testContext.database)
	found := newStoredClaim(t, consumerID, request.ID, claim.PartyConsumer, uuid.NewString())
	found.Actions = append(found.Actions, claim.Action{Type: "submitted", ActorID: 2147483647, ActorParty: claim.PartyConsumer, CreatedOn: found.CreatedOn})
	require.Error(t, repository.Save(t.Context(), found))
	require.Zero(t, found.ID)
	for _, table := range []string{"claims", "claim_actions", "claim_images", "claim_resolutions"} {
		var count int
		require.NoError(t, fixture.testContext.database.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&count))
		require.Zero(t, count)
	}
	absent, err := repository.FindBySubmissionKey(t.Context(), consumerID, found.SubmissionKey())
	require.NoError(t, err)
	require.Nil(t, absent)
	found.Actions = found.Actions[:1]
	require.NoError(t, repository.Save(t.Context(), found))
}
func TestClaimRepositoryIsolationPaginationAndStateFilter(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	repository := repositories.NewClaimRepository(fixture.testContext.database)
	ids := make([]int, 0, 3)
	request := fixture.jobRequest(t, consumerID, providerID, time.Now(), "pending")
	for i := 0; i < 3; i++ {
		proposalID := fixture.proposal(t, request, time.Now(), "pending")
		found := newStoredClaim(t, consumerID, request.ID, claim.PartyConsumer, uuid.NewString())
		if i > 0 {
			found.OperationID = operationmodel.ID{Kind: operationmodel.KindServiceProposal, ResourceID: proposalID}
		}
		require.NoError(t, repository.Save(t.Context(), found))
		ids = append(ids, found.ID)
		counterpart := newStoredClaim(t, providerID, request.ID, claim.PartyProvider, found.SubmissionKey())
		counterpart.OperationID = found.OperationID
		require.NoError(t, repository.Save(t.Context(), counterpart))
	}
	open := claim.StatusOpen
	page, err := repository.FindOwnedPage(t.Context(), consumerID, claim.ListCriteria{Status: &open, Page: 1, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, 3, page.Total)
	require.Equal(t, ids[2], page.Claims[0].ID)
	require.Equal(t, ids[1], page.Claims[1].ID)
	page, err = repository.FindOwnedPage(t.Context(), consumerID, claim.ListCriteria{Page: 3, Limit: 2})
	require.NoError(t, err)
	require.NotNil(t, page.Claims)
	require.Empty(t, page.Claims)
	require.Equal(t, 3, page.Total)
	dismissed := claim.StatusDismissed
	page, err = repository.FindOwnedPage(t.Context(), consumerID, claim.ListCriteria{Status: &dismissed})
	require.NoError(t, err)
	require.Zero(t, page.Total)
	hidden, err := repository.FindOwnedByID(t.Context(), providerID, ids[0])
	require.NoError(t, err)
	require.Nil(t, hidden)
	hidden, err = repository.FindBySubmissionKey(t.Context(), 2147483647, uuid.NewString())
	require.NoError(t, err)
	require.Nil(t, hidden)
}
func TestClaimRepositoryConcurrentSubmissionConstraints(t *testing.T) {
	for _, mode := range []string{"same_key_same_content", "same_key_different_content", "different_keys_same_operation"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newOperationInboxFixture(t)
			consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
			request := fixture.jobRequest(t, consumerID, providerID, time.Now(), "pending")
			repository := repositories.NewClaimRepository(fixture.testContext.database)
			key := uuid.NewString()
			first := newStoredClaim(t, consumerID, request.ID, claim.PartyConsumer, key)
			second := newStoredClaim(t, consumerID, request.ID, claim.PartyConsumer, key)
			if mode == "different_keys_same_operation" {
				second = newStoredClaim(t, consumerID, request.ID, claim.PartyConsumer, uuid.NewString())
			}
			if mode == "same_key_different_content" {
				second.Description = "different testimony"
				input := claim.Submission{Reference: second.Reference, Reason: second.Reason, Description: second.Description, ImageFileIDs: second.ImageFileIDs}
				second = claim.Rehydrate(*second, key, input.Fingerprint())
			}
			start := make(chan struct{})
			errs := make(chan error, 2)
			var wg sync.WaitGroup
			for _, found := range []*claim.Claim{first, second} {
				wg.Go(func() { <-start; errs <- repository.Save(t.Context(), found) })
			}
			close(start)
			wg.Wait()
			close(errs)
			success, conflicts := 0, 0
			for err := range errs {
				if err == nil {
					success++
				} else {
					require.True(t, errors.Is(err, claim.ErrPersistenceConflict) || errors.Is(err, claim.ErrOpenClaimConflict))
					conflicts++
				}
			}
			require.Equal(t, 1, success)
			require.Equal(t, 1, conflicts)
			var count int
			require.NoError(t, fixture.testContext.database.QueryRow(`SELECT COUNT(*) FROM claims`).Scan(&count))
			require.Equal(t, 1, count)
			require.NoError(t, fixture.testContext.database.QueryRow(`SELECT COUNT(*) FROM claim_actions`).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}
func TestClaimOperationReferenceResolverReusesCanonicalIdentityAndRestrictsPayments(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	base := time.Now().UTC()
	request := fixture.jobRequest(t, consumerID, providerID, base, "accepted")
	first := fixture.proposal(t, request, base, "accepted")
	later := fixture.proposal(t, request, base.Add(time.Minute), "accepted")
	firstOrder := fixture.workOrder(t, first, base, "scheduled")
	laterOrder := fixture.workOrder(t, later, base, "scheduled")
	resolver := repositories.NewClaimOperationReferenceResolver(fixture.testContext.database)
	pi := uuid.NewString()
	_, err := fixture.testContext.database.Exec(`INSERT INTO payment_intents(id,service_proposal_id,purpose,currency,seller_amount_cents,platform_fee_cents,total_amount_cents,status,created_on,updated_on) VALUES($1,$2,'booking_deposit','ARS',100,10,110,'rejected',$3,$3)`, pi, later, base)
	require.NoError(t, err)
	cases := []struct {
		kind     claim.ReferenceKind
		id       string
		expected operationmodel.ID
	}{{claim.ReferenceKindJobRequest, strconv.Itoa(request.ID), operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}}, {claim.ReferenceKindServiceProposal, strconv.Itoa(first), operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}}, {claim.ReferenceKindServiceProposal, strconv.Itoa(later), operationmodel.ID{Kind: operationmodel.KindServiceProposal, ResourceID: later}}, {claim.ReferenceKindWorkOrder, strconv.Itoa(firstOrder), operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}}, {claim.ReferenceKindWorkOrder, strconv.Itoa(laterOrder), operationmodel.ID{Kind: operationmodel.KindServiceProposal, ResourceID: later}}, {claim.ReferenceKindPaymentIntent, pi, operationmodel.ID{Kind: operationmodel.KindServiceProposal, ResourceID: later}}}
	for _, test := range cases {
		t.Run(fmt.Sprintf("%s_%s", test.kind, test.id), func(t *testing.T) {
			reference, err := claim.NewReference(test.kind, test.id)
			require.NoError(t, err)
			found, err := resolver.ResolveClaimOperationReference(t.Context(), claim.Claimant{ID: consumerID, Party: claim.PartyConsumer}, reference)
			require.NoError(t, err)
			require.Equal(t, test.expected, *found)
			found, err = resolver.ResolveClaimOperationReference(t.Context(), claim.Claimant{ID: providerID, Party: claim.PartyProvider}, reference)
			require.NoError(t, err)
			if test.kind == claim.ReferenceKindPaymentIntent {
				require.Nil(t, found)
			} else {
				require.Equal(t, test.expected, *found)
			}
			found, err = resolver.ResolveClaimOperationReference(t.Context(), claim.Claimant{ID: 2147483647, Party: claim.PartyConsumer}, reference)
			require.NoError(t, err)
			require.Nil(t, found)
		})
	}
}
func TestClaimReadersPropagateCancellationAndDatabaseFailures(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	repository := repositories.NewClaimRepository(fixture.testContext.database)
	_, err := repository.FindOwnedByID(ctx, 1, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = repository.FindOwnedPage(ctx, 1, claim.ListCriteria{})
	require.ErrorIs(t, err, context.Canceled)
	database, err := sql.Open("pgx", "postgres://unused")
	require.NoError(t, err)
	require.NoError(t, database.Close())
	closed := repositories.NewClaimRepository(database)
	_, err = closed.FindOwnedByID(t.Context(), 1, 1)
	require.Error(t, err)
	_, err = closed.FindOwnedPage(t.Context(), 1, claim.ListCriteria{})
	require.Error(t, err)
	_, err = repositories.NewClaimUserFinder(database).FindClaimantByAuthID(t.Context(), "auth")
	require.Error(t, err)
}
