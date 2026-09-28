package admin_handler

import (
	"encoding/json"
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConsumerHistoryHandlerErrorsDoNotDeliverPrivateData(t *testing.T) {
	for _, tc := range []struct {
		err     error
		code    int
		message string
	}{{admin.ErrConsumerHistoryNotFound, 404, "consumer not found"}, {admin.ErrInvalidConsumerHistoryQuery, 400, "invalid consumer history query"}, {errors.New("private audit credentials"), 500, "internal server error"}} {
		s := new(consumerHistoryServiceMock)
		s.On("Query", mock.Anything, 12, admin.ConsumerHistoryQuery{Limit: 20}, "support", "history-test").Return(nil, tc.err).Once()
		h, err := NewConsumerHistoryHandler(s, []byte(strings.Repeat("k", 32)))
		require.NoError(t, err)
		r := gin.New()
		r.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
		r.GET("/admin/consumers/:consumer_id/history", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "support") }, h.Get)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/admin/consumers/12/history", nil)
		req.Header.Set("X-Request-ID", "history-test")
		r.ServeHTTP(rec, req)
		require.Equal(t, tc.code, rec.Code)
		require.JSONEq(t, `{"error":"`+tc.message+`"}`, rec.Body.String())
		s.AssertExpectations(t)
	}
}
func TestConsumerHistoryHandlerInvalidInputDoesNotCallService(t *testing.T) {
	s := new(consumerHistoryServiceMock)
	h, err := NewConsumerHistoryHandler(s, []byte(strings.Repeat("k", 32)))
	require.NoError(t, err)
	r := gin.New()
	r.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	r.GET("/admin/consumers/:consumer_id/history", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "support") }, h.Get)
	for _, path := range []string{"/admin/consumers/0/history", "/admin/consumers/2147483648/history", "/admin/consumers/12/history?status=paid", "/admin/consumers/12/history?limit=1000"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 400, rec.Code)
	}
	s.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
func TestConsumerHistoryMapperProjectsExactResourceVariants(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.FixedZone("offset", -3*3600))
	requestID := 7
	h := &readmodel.ConsumerHistory{Consumer: readmodel.Consumer{ID: 12, Name: "Ana", Surname: "Perez", Email: "ana@example.com", CreatedOn: now, ProfilePhotoFileID: "internal-secret-photo"}, Address: &readmodel.ConsumerHistoryAddress{Street: "Rivadavia", StreetNumber: "5100"}, CoverageZone: &readmodel.ConsumerHistoryZone{ID: 3, Name: "Caballito", Enabled: true}, Summary: readmodel.ConsumerHistorySummary{JobRequests: 5, ServiceProposals: 6, WorkOrders: 7}}
	for _, typ := range []string{"job_request", "service_proposal", "work_order"} {
		h.Items = append(h.Items, readmodel.ConsumerHistoryItem{Type: typ, ID: 7, Status: "accepted", Provider: operationmodel.Party{ID: 23, Name: "Juan", Surname: "Gomez"}, OccurredOn: now, CreatedOn: now, ScheduledOn: now, AcceptedOn: now, JobRequestID: &requestID, ServiceProposalID: 8, EstimatedDurationMinutes: 60, BookingPaymentDeadline: now, Operation: operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: 7}})
	}
	data, err := json.Marshal(consumerHistoryResponseFromModel(h, 20))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	require.ElementsMatch(t, []string{"consumer", "summary", "page"}, historyResponseKeys(body))
	consumer := body["consumer"].(map[string]any)
	require.ElementsMatch(t, []string{"id", "role", "name", "surname", "email", "profile_photo_url", "created_on", "address", "coverage_zone"}, historyResponseKeys(consumer))
	require.Nil(t, consumer["profile_photo_url"])
	require.Equal(t, "2026-09-28T15:00:00Z", consumer["created_on"])
	address := consumer["address"].(map[string]any)
	require.ElementsMatch(t, []string{"street", "street_number", "floor", "unit", "source"}, historyResponseKeys(address))
	require.Nil(t, address["floor"])
	require.Nil(t, address["unit"])
	require.Equal(t, "current_consumer_profile", address["source"])
	page := body["page"].(map[string]any)
	require.ElementsMatch(t, []string{"items", "limit", "next_cursor"}, historyResponseKeys(page))
	require.Nil(t, page["next_cursor"])
	items := page["items"].([]any)
	common := []string{"type", "id", "status", "provider", "occurred_on", "operation"}
	variants := [][]string{{"created_on"}, {"job_request_id", "created_on", "scheduled_on", "estimated_duration_minutes", "booking_payment_deadline"}, {"job_request_id", "service_proposal_id", "accepted_on", "completion_reported_on", "balance_paid_on"}}
	for j, item := range items {
		v := item.(map[string]any)
		keys := append(append([]string{}, common...), variants[j]...)
		require.ElementsMatch(t, keys, historyResponseKeys(v))
		party := v["provider"].(map[string]any)
		require.ElementsMatch(t, []string{"id", "name", "surname"}, historyResponseKeys(party))
		operation := v["operation"].(map[string]any)
		require.ElementsMatch(t, []string{"id", "url", "required_permission", "chat_required_permission"}, historyResponseKeys(operation))
		require.Equal(t, "jr-7", operation["id"])
		require.Equal(t, "/admin/operations/jr-7", operation["url"])
		require.Equal(t, "read:admin_operations", operation["required_permission"])
		require.Equal(t, "read:admin_chat_audit", operation["chat_required_permission"])
	}
	require.Nil(t, items[2].(map[string]any)["completion_reported_on"])
	require.Nil(t, items[2].(map[string]any)["balance_paid_on"])
	require.NotContains(t, string(data), "internal-secret-photo")
	empty, err := json.Marshal(consumerHistoryResponseFromModel(&readmodel.ConsumerHistory{}, 20))
	require.NoError(t, err)
	require.Contains(t, string(empty), `"items":[]`)
	require.Contains(t, string(empty), `"address":null`)
	require.Contains(t, string(empty), `"coverage_zone":null`)
}
func historyResponseKeys(m map[string]any) []string {
	result := make([]string, 0, len(m))
	for key := range m {
		result = append(result, key)
	}
	return result
}
func TestConsumerHistoryHandlerEmitsBoundNextCursor(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	s := new(consumerHistoryServiceMock)
	q := admin.ConsumerHistoryQuery{Limit: 2, ProviderID: 23}
	model := &readmodel.ConsumerHistory{Consumer: readmodel.Consumer{ID: 12}, HasMore: true, Items: []readmodel.ConsumerHistoryItem{{Type: "job_request", ID: 9, OccurredOn: now, CreatedOn: now}, {Type: "job_request", ID: 8, OccurredOn: now, CreatedOn: now}}}
	s.On("Query", mock.Anything, 12, q, "support", "history-test").Return(model, nil).Once()
	h, err := NewConsumerHistoryHandler(s, []byte(strings.Repeat("k", 32)))
	require.NoError(t, err)
	r := gin.New()
	r.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	r.GET("/admin/consumers/:consumer_id/history", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "support") }, h.Get)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/consumers/12/history?limit=2&provider_id=23", nil)
	req.Header.Set("X-Request-ID", "history-test")
	r.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	var body consumerHistoryResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Page.NextCursor)
	next, err := parseConsumerHistoryQuery("cursor="+*body.Page.NextCursor, 12, h.cursors)
	require.NoError(t, err)
	require.Equal(t, 2, next.Limit)
	require.Equal(t, 23, next.ProviderID)
	require.Equal(t, 8, next.After.ID)
	require.Equal(t, now, next.After.OccurredOn)
}
