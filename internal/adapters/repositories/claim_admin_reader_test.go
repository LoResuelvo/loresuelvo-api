package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newAdministrativeClaimPageFixture(t *testing.T) (*repositories.ClaimRepository, []int, []int) {
	t.Helper()
	fixture := newOperationInboxFixture(t)
	consumer, provider := savedJobRequestParticipants(t, fixture.testContext)
	ctx := context.Background()
	repo := repositories.NewClaimRepository(fixture.testContext.database)
	base := time.Date(2026, 9, 24, 13, 0, 0, 0, time.UTC)
	ids := make([]int, 3)
	operations := make([]int, 3)
	for i := range 3 {
		date := base
		if i == 2 {
			date = base.Add(-24 * time.Hour)
		}
		currentProvider := provider
		if i > 0 {
			currentProvider = savedProviderIDWithData(t, fixture.testContext, fmt.Sprintf("auth0|page-provider-%d", i), fmt.Sprintf("page-provider-%d@example.com", i), "Provider", "Local", "Plomeria")
		}
		request := fixture.jobRequest(t, consumer, currentProvider, date.Add(-time.Hour), "pending")
		operations[i] = request.ID
		ref, err := claim.NewReference(claim.ReferenceKindJobRequest, fmt.Sprint(request.ID))
		require.NoError(t, err)
		submission := claim.Submission{Reference: ref, Reason: claim.ReasonDamage, Description: "sensitive original"}
		if i == 0 {
			for range 2 {
				id := uuid.NewString()
				_, err := fixture.testContext.database.Exec(`INSERT INTO files(id,key,bucket,original_name,mime_type,size_bytes,status,visibility,purpose,uploaded_by_auth_id,created_on,updated_on) VALUES($1,$2,'private','image.png','image/png',32,'confirmed','private','claim_evidence_image','claimant',NOW(),NOW())`, id, id)
				require.NoError(t, err)
				submission.ImageFileIDs = append(submission.ImageFileIDs, id)
			}
		}
		found, err := claim.New(claim.Claimant{ID: consumer, Party: claim.PartyConsumer}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, uuid.NewString(), submission, date)
		require.NoError(t, err)
		require.NoError(t, repo.Save(ctx, found))
		ids[i] = found.ID
	}
	return repo, ids, operations
}

func TestAdministrativeClaimPageOrdersAndCountsWithoutEvidenceInflation(t *testing.T) {
	repo, ids, _ := newAdministrativeClaimPageFixture(t)
	ctx := context.Background()
	status := claim.StatusOpen
	page, err := repo.FindAdministrativePage(ctx, claim.AdminCriteria{ListCriteria: claim.ListCriteria{Status: &status, Page: 1, Limit: 2}})
	require.NoError(t, err)
	require.Equal(t, 3, page.Total)
	require.Len(t, page.Claims, 2)
	require.Equal(t, ids[1], page.Claims[0].ID)
	require.Equal(t, ids[0], page.Claims[1].ID)
	require.NotNil(t, page.Claims[0].CategoryID)
	for _, item := range page.Claims {
		require.Equal(t, "consumer", item.ClaimantParty)
		require.Equal(t, "job.request.consumer@example.com", item.ClaimantEmail)
	}
	second, err := repo.FindAdministrativePage(ctx, claim.AdminCriteria{ListCriteria: claim.ListCriteria{Page: 2, Limit: 2}})
	require.NoError(t, err)
	require.Len(t, second.Claims, 1)
	require.Equal(t, ids[2], second.Claims[0].ID)
	empty, err := repo.FindAdministrativePage(ctx, claim.AdminCriteria{ListCriteria: claim.ListCriteria{Page: 3, Limit: 2}})
	require.NoError(t, err)
	require.NotNil(t, empty.Claims)
	require.Empty(t, empty.Claims)
	require.Equal(t, 3, empty.Total)
}

func TestAdministrativeClaimPageCombinesSearchAndState(t *testing.T) {
	repo, _, operations := newAdministrativeClaimPageFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		query string
		total int
	}{
		{"job.request.consumer@example.com", 3}, {"REQUEST.CONSUMER@", 3}, {fmt.Sprintf("jr-%d", operations[0]), 1}, {"%", 0}, {"_", 0}, {"' OR 1=1 --", 0},
	} {
		filtered, err := repo.FindAdministrativePage(ctx, claim.AdminCriteria{Query: tc.query})
		require.NoError(t, err)
		require.Equal(t, tc.total, filtered.Total, tc.query)
	}
	resolved := claim.StatusResolved
	none, err := repo.FindAdministrativePage(ctx, claim.AdminCriteria{ListCriteria: claim.ListCriteria{Status: &resolved}, Query: "job.request.consumer@example.com"})
	require.NoError(t, err)
	require.Empty(t, none.Claims)
	require.Zero(t, none.Total)
}

func TestAdministrativeClaimDetailHydratesOwnedDataAndContext(t *testing.T) {
	repo, ids, _ := newAdministrativeClaimPageFixture(t)
	ctx := context.Background()
	detail, err := repo.FindAdministrativeByID(ctx, ids[0])
	require.NoError(t, err)
	require.Equal(t, "sensitive original", detail.Claim.Description)
	require.Len(t, detail.Claim.ImageFileIDs, 2)
	require.Len(t, detail.Claim.Actions, 1)
	require.Equal(t, "job.request.consumer@example.com", detail.ClaimantEmail)
}

func TestAdministrativeClaimDetailReturnsAbsentResource(t *testing.T) {
	repo, _, _ := newAdministrativeClaimPageFixture(t)
	ctx := context.Background()
	missing, err := repo.FindAdministrativeByID(ctx, 2147483647)
	require.NoError(t, err)
	require.Nil(t, missing)
}
