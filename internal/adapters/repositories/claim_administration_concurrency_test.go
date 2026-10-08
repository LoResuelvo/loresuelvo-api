package repositories_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/claim_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestClaimAdministrationConcurrentRequestsWithPostgres(t *testing.T) {
	for _, mode := range []string{"same key normalized content", "same key different content", "different keys same claim", "same key different claims", "same key different actions"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newOperationInboxFixture(t)
			consumer, provider := savedJobRequestParticipants(t, fixture.testContext)
			now := time.Now().UTC().Truncate(time.Microsecond)
			request := fixture.jobRequest(t, consumer, provider, now.Add(-time.Hour), "pending")
			operator, err := admin.NewAdmin("auth0|claim-operator", "operator@example.com", "Operator", "Local", nil)
			require.NoError(t, err)
			_, err = fixture.testContext.userRepository.Save(context.Background(), operator)
			require.NoError(t, err)
			database := fixture.testContext.database
			repo := repositories.NewClaimRepository(database)
			events := repositories.NewAuditEventRepository(database)
			var unit claim.AdministrationUnitOfWork = repositories.NewClaimAdministrationUnitOfWork(database, repo, events)
			var barrier *administrationRecordReadBarrier
			if mode == "same key different claims" {
				barrier = &administrationRecordReadBarrier{unit: unit, ready: make(chan struct{})}
				unit = barrier
			}
			svc := claim.NewAdminService(repo, fixture.testContext.userRepository, unit, events, nil, clockadapter.NewSystemClock())
			reference, err := claim.NewReference(claim.ReferenceKindJobRequest, fmt.Sprint(request.ID))
			require.NoError(t, err)
			create := func(id int) *claim.Claim {
				found, err := claim.New(claim.Claimant{ID: id, Party: claim.PartyConsumer}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, uuid.NewString(), claim.Submission{Reference: reference, Reason: claim.ReasonDamage, Description: "Original testimony"}, now.Add(-time.Minute))
				require.NoError(t, err)
				_, err = found.StartReview(operator.ID(), now)
				require.NoError(t, err)
				require.NoError(t, repo.Save(context.Background(), found))
				return found
			}
			first := create(consumer)
			ids := [2]int{first.ID, first.ID}
			keys := [2]string{uuid.NewString(), ""}
			keys[1] = keys[0]
			bodies := [2]string{`{"type":"agreement","reasoning":"valid"}`, `{"type":"agreement","reasoning":"  valid  "}`}
			routes := [2]string{"resolution", "resolution"}
			switch mode {
			case "same key different content":
				bodies[1] = `{"type":"provider_favor","reasoning":"different"}`
			case "different keys same claim":
				keys[1] = uuid.NewString()
			case "same key different claims":
				ids[1] = create(provider).ID
			case "same key different actions":
				// This review key is genuinely persisted through the service before its resolution retry.
				open, err := claim.New(claim.Claimant{ID: provider, Party: claim.PartyProvider}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, uuid.NewString(), claim.Submission{Reference: reference, Reason: claim.ReasonDamage, Description: "Original"}, now.Add(-time.Minute))
				require.NoError(t, err)
				require.NoError(t, repo.Save(context.Background(), open))
				_, err = svc.StartReview(context.Background(), "auth0|claim-operator", open.ID, keys[0], "original-review")
				require.NoError(t, err)
				ids = [2]int{open.ID, open.ID}
				routes[0] = "review"
				bodies[0] = ""
			}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth0|claim-operator") })
			handler := claim_handler.NewAdminHandler(svc)
			router.POST("/admin/claims/:id/resolution", handler.Resolve)
			router.POST("/admin/claims/:id/review", handler.StartReview)
			results := [2]*httptest.ResponseRecorder{}
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := range 2 {
				wg.Go(func() {
					<-start
					req := httptest.NewRequest("POST", fmt.Sprintf("/admin/claims/%d/%s", ids[i], routes[i]), strings.NewReader(bodies[i]))
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					req = req.WithContext(ctx)
					req.Header.Set("Idempotency-Key", keys[i])
					results[i] = httptest.NewRecorder()
					router.ServeHTTP(results[i], req)
				})
			}
			close(start)
			wg.Wait()
			codes := []int{results[0].Code, results[1].Code}
			slices.Sort(codes)
			expected := []int{200, 409}
			if mode == "same key normalized content" {
				expected = []int{200, 200}
			}
			require.Equal(t, expected, codes, "responses %s; %s", results[0].Body.String(), results[1].Body.String())
			if mode == "same key normalized content" {
				var a, b struct {
					Action struct {
						ID        int       `json:"id"`
						CreatedOn time.Time `json:"created_on"`
					} `json:"action"`
				}
				require.NoError(t, json.Unmarshal(results[0].Body.Bytes(), &a))
				require.NoError(t, json.Unmarshal(results[1].Body.Bytes(), &b))
				require.Equal(t, a, b)
				require.Positive(t, a.Action.ID)
			}
			if barrier != nil {
				require.Equal(t, int32(1), barrier.conflicts.Load(), "one real uniqueness collision must rollback before replaying the winning record")
			}
			var count int
			require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM claim_administration_records WHERE operator_id=$1`, operator.ID()).Scan(&count))
			require.Equal(t, 1, count)
			require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE operator_id=$1 AND resource_type='claim' AND action='execute'`, operator.ID()).Scan(&count))
			require.Equal(t, 1, count)
			if mode != "same key different actions" {
				require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM claim_resolutions`).Scan(&count))
				require.Equal(t, 1, count)
			}
		})
	}
}

func TestClaimAdministrationWaitingReplayHydratesOriginalResolutionWithPostgres(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumer, provider := savedJobRequestParticipants(t, fixture.testContext)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Microsecond)
	request := fixture.jobRequest(t, consumer, provider, now.Add(-time.Hour), "pending")
	operator, err := admin.NewAdmin("auth0|claim-operator", "operator@example.com", "Operator", "Local", nil)
	require.NoError(t, err)
	_, err = fixture.testContext.userRepository.Save(ctx, operator)
	require.NoError(t, err)
	db := fixture.testContext.database
	repo := repositories.NewClaimRepository(db)
	events := repositories.NewAuditEventRepository(db)
	ref, err := claim.NewReference(claim.ReferenceKindJobRequest, fmt.Sprint(request.ID))
	require.NoError(t, err)
	found, err := claim.New(claim.Claimant{ID: consumer, Party: claim.PartyConsumer}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, uuid.NewString(), claim.Submission{Reference: ref, Reason: claim.ReasonDamage, Description: "original"}, now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = found.StartReview(operator.ID(), now)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, found))
	barrier := &administrationCommitBarrier{unit: repositories.NewClaimAdministrationUnitOfWork(db, repo, events), prepared: make(chan struct{}), release: make(chan struct{})}
	svc := claim.NewAdminService(repo, fixture.testContext.userRepository, barrier, events, nil, clockadapter.NewSystemClock())
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth0|claim-operator") })
	router.POST("/admin/claims/:id/resolution", claim_handler.NewAdminHandler(svc).Resolve)
	key := uuid.NewString()
	results := [2]*httptest.ResponseRecorder{}
	var wg sync.WaitGroup
	var release sync.Once
	defer func() { release.Do(func() { close(barrier.release) }); wg.Wait() }()
	invoke := func(i int) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/admin/claims/%d/resolution", found.ID), strings.NewReader(`{"type":"agreement","reasoning":"  verified  ","suggested_compensation":{"amount_minor":1500}}`)).WithContext(ctx)
		req.Header.Set("Idempotency-Key", key)
		results[i] = httptest.NewRecorder()
		router.ServeHTTP(results[i], req)
	}
	wg.Go(func() { invoke(0) })
	select {
	case <-barrier.prepared:
	case <-ctx.Done():
		t.Fatal("original resolution did not reach the commit barrier")
	}
	wg.Go(func() { invoke(1) })
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND wait_event_type='Lock' AND query LIKE '%claims%' AND query LIKE '%FOR UPDATE%')`).Scan(&waiting)
		require.NoError(t, err)
		return waiting
	}, 5*time.Second, 10*time.Millisecond, "replay must acquire a statement snapshot before waiting for the original commit")
	release.Do(func() { close(barrier.release) })
	wg.Wait()
	type resolution struct {
		Type                  string    `json:"type"`
		Reasoning             string    `json:"reasoning"`
		ResolvedOn            time.Time `json:"resolved_on"`
		SuggestedCompensation struct {
			AmountMinor int64  `json:"amount_minor"`
			Currency    string `json:"currency"`
			Unit        string `json:"unit"`
		} `json:"suggested_compensation"`
	}
	type outcome struct {
		Status string `json:"status"`
		Action struct {
			ID         int       `json:"id"`
			ActorID    int       `json:"actor_id"`
			ActorParty string    `json:"actor_party"`
			CreatedOn  time.Time `json:"created_on"`
		} `json:"action"`
		Resolution *resolution `json:"resolution"`
	}
	outcomes := [2]outcome{}
	for i := range 2 {
		require.Equal(t, 200, results[i].Code, results[i].Body.String())
		require.NoError(t, json.Unmarshal(results[i].Body.Bytes(), &outcomes[i]))
	}
	require.NotNil(t, outcomes[0].Resolution)
	require.NotNil(t, outcomes[1].Resolution, "a waiting replay must not lose the concurrently committed dictamen")
	require.Equal(t, outcomes[0], outcomes[1])
	require.Equal(t, "agreement", outcomes[1].Resolution.Type)
	require.Equal(t, "verified", outcomes[1].Resolution.Reasoning)
	require.Equal(t, int64(1500), outcomes[1].Resolution.SuggestedCompensation.AmountMinor)
	require.Equal(t, "ARS", outcomes[1].Resolution.SuggestedCompensation.Currency)
	require.Equal(t, "minor", outcomes[1].Resolution.SuggestedCompensation.Unit)
	require.Equal(t, operator.ID(), outcomes[1].Action.ActorID)
	require.Equal(t, "operator", outcomes[1].Action.ActorParty)
	require.Equal(t, outcomes[1].Action.CreatedOn, outcomes[1].Resolution.ResolvedOn)
}
