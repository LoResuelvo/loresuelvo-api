package repositories_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/claim_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestClaimSuggestedCompensationPreservesFinancialRowsWithPostgres(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumer, provider := savedJobRequestParticipants(t, fixture.testContext)
	ctx := t.Context()
	db := fixture.testContext.database
	now := time.Now().UTC().Truncate(time.Microsecond)
	request := fixture.jobRequest(t, consumer, provider, now.Add(-4*time.Hour), "accepted")
	conversation, err := fixture.testContext.conversationRepository.FindByID(ctx, request.ConversationID)
	require.NoError(t, err)
	require.NoError(t, conversation.Activate())
	_, err = fixture.testContext.conversationRepository.SaveConversation(ctx, conversation)
	require.NoError(t, err)
	proposalID := fixture.scheduledProposal(t, request, now.Add(-3*time.Hour), now.Add(48*time.Hour), 60, "accepted")
	fixture.workOrder(t, proposalID, now.Add(-2*time.Hour), "scheduled")
	proposal, err := repositories.NewServiceProposalRepository(db).FindByID(ctx, proposalID)
	require.NoError(t, err)
	terms := proposal.BookingTerms
	require.Equal(t, "ARS", terms.Currency())
	require.Positive(t, terms.AmountDueNowCents())
	saveCollectionPayment(t, serviceProposalRepositoryTestContext{database: db}, proposalID, string(payment.PurposeBookingDeposit), terms.DepositCents(), terms.PlatformFeeDueNowCents(), now.Add(-2*time.Hour))
	operator, err := admin.NewAdmin("auth0|claim-operator", "operator@example.com", "Operator", "Local", nil)
	require.NoError(t, err)
	_, err = fixture.testContext.userRepository.Save(ctx, operator)
	require.NoError(t, err)
	claims := repositories.NewClaimRepository(db)
	events := repositories.NewAuditEventRepository(db)
	service := claim.NewAdminService(claims, fixture.testContext.userRepository, repositories.NewClaimAdministrationUnitOfWork(db, claims, events), events, nil, clockadapter.NewSystemClock())
	reference, err := claim.NewReference(claim.ReferenceKindJobRequest, fmt.Sprint(request.ID))
	require.NoError(t, err)
	found, err := claim.New(claim.Claimant{ID: consumer, Party: claim.PartyConsumer}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, uuid.NewString(), claim.Submission{Reference: reference, Reason: claim.ReasonDamage, Description: "Original testimony"}, now.Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, claims.Save(ctx, found))
	_, err = service.StartReview(ctx, operator.AuthID(), found.ID, uuid.NewString(), "compensation-review")
	require.NoError(t, err)

	// Compare all stored columns and row counts, not only an empty payment collection or selected amounts.
	captureFinancialRows := func() string {
		var rows string
		err := db.QueryRowContext(ctx, `SELECT jsonb_build_object(
   'proposals',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM service_proposals p),
   'orders',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM work_orders o),
   'intents',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id) FROM payment_intents i),
   'transactions',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM payment_transactions t))`).Scan(&rows)
		require.NoError(t, err)
		return rows
	}
	before := captureFinancialRows()
	var rows map[string][]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(before), &rows))
	for _, table := range []string{"proposals", "orders", "intents", "transactions"} {
		require.Len(t, rows[table], 1, "the financial fixture must be non-empty and unambiguous: %s", table)
	}
	intent, err := repositories.NewPaymentIntentRepository(db).FindLatestByProposalIDAndPurpose(ctx, proposalID, payment.PurposeBookingDeposit)
	require.NoError(t, err)
	require.Equal(t, payment.StatusPaid, intent.Status)
	require.Equal(t, terms.AmountDueNowCents(), intent.TotalAmountCents)
	// The constructor's field names differ from SQL; inspect the persisted evidence using its JSON column names.
	var evidence struct {
		Status   string `json:"status"`
		Currency string `json:"currency"`
		Amount   int64  `json:"amount_cents"`
	}
	require.NoError(t, json.Unmarshal(rows["transactions"][0], &evidence))
	require.Equal(t, "approved", evidence.Status)
	require.Equal(t, "ARS", evidence.Currency)
	require.Equal(t, terms.AmountDueNowCents(), evidence.Amount)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, operator.AuthID()) })
	router.POST("/admin/claims/:id/resolution", claim_handler.NewAdminHandler(service).Resolve)
	req := httptest.NewRequest("POST", fmt.Sprintf("/admin/claims/%d/resolution", found.ID), strings.NewReader(`{"type":"agreement","reasoning":"Verified agreement","suggested_compensation":{"amount_minor":1500}}`)).WithContext(ctx)
	req.Header.Set("Idempotency-Key", uuid.NewString())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	require.Equal(t, 200, response.Code, response.Body.String())
	persisted, err := claims.FindAdministrativeByID(ctx, found.ID)
	require.NoError(t, err)
	require.Equal(t, claim.StatusResolved, persisted.Claim.Status)
	require.Equal(t, int64(1500), persisted.Claim.Resolution.SuggestedCompensation.AmountMinor)
	require.JSONEq(t, before, captureFinancialRows(), "a suggested compensation must not change or add financial records")
}
