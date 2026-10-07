package repositories_test

import (
	"encoding/json"
	"fmt"
	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/claim_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClaimServiceConcurrentSubmissionWithPostgres(t *testing.T) {
	for _, mode := range []string{"same key same content", "same key different content", "different keys same operation", "same key different claimants"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newOperationInboxFixture(t)
			consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
			request := fixture.jobRequest(t, consumerID, providerID, time.Now(), "pending")
			database := fixture.testContext.database
			repo := repositories.NewClaimRepository(database)
			svc := claim.NewService(repo, repositories.NewClaimUserFinder(database), repositories.NewClaimOperationReferenceResolver(database), nil, clockadapter.NewSystemClock())
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, c.GetHeader("Test-Auth-ID")) })
			handler := claim_handler.NewHandler(svc)
			router.POST("/claims", handler.Submit)
			keys := [2]string{uuid.NewString(), ""}
			keys[1] = keys[0]
			descriptions := [2]string{"original testimony", "original testimony"}
			actors := [2]string{"auth0|job-request-consumer", "auth0|job-request-consumer"}
			switch mode {
			case "same key different content":
				descriptions[1] = "different testimony"
			case "different keys same operation":
				keys[1] = uuid.NewString()
			case "same key different claimants":
				actors[1] = "auth0|job-request-provider"
			}
			responses := [2]*httptest.ResponseRecorder{}
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := range 2 {
				wg.Go(func() {
					<-start
					req := httptest.NewRequest("POST", "/claims", strings.NewReader(fmt.Sprintf(`{"reference":{"job_request_id":%d},"reason":"damage","description":%q}`, request.ID, descriptions[i])))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Idempotency-Key", keys[i])
					req.Header.Set("Test-Auth-ID", actors[i])
					responses[i] = httptest.NewRecorder()
					router.ServeHTTP(responses[i], req)
				})
			}
			close(start)
			wg.Wait()
			codes := []int{responses[0].Code, responses[1].Code}
			slices.Sort(codes)
			expected := []int{201, 409}
			if mode == "same key same content" {
				expected = []int{200, 201}
			}
			if mode == "same key different claimants" {
				expected = []int{201, 201}
			}
			require.Equal(t, expected, codes, "responses: %s; %s", responses[0].Body.String(), responses[1].Body.String())
			if mode == "same key same content" {
				var first, second struct {
					ID        int       `json:"id"`
					CreatedOn time.Time `json:"created_on"`
				}
				require.NoError(t, json.Unmarshal(responses[0].Body.Bytes(), &first))
				require.NoError(t, json.Unmarshal(responses[1].Body.Bytes(), &second))
				require.Positive(t, first.ID)
				require.Equal(t, first.ID, second.ID)
				require.Equal(t, first.CreatedOn, second.CreatedOn)
				require.Equal(t, 0, first.CreatedOn.Nanosecond()%1000)
			}
			if mode == "same key different content" || mode == "different keys same operation" {
				loser := responses[0]
				if loser.Code != 409 {
					loser = responses[1]
				}
				expectedError := claim.ErrSubmissionKeyConflict
				if mode == "different keys same operation" {
					expectedError = claim.ErrOpenClaimConflict
				}
				require.JSONEq(t, fmt.Sprintf(`{"error":%q}`, expectedError.Error()), loser.Body.String())
			}
			countExpected := 1
			if mode == "same key different claimants" {
				countExpected = 2
			}
			for _, table := range []string{"claims", "claim_actions"} {
				var count int
				require.NoError(t, database.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count))
				require.Equal(t, countExpected, count)
			}
		})
	}
}
