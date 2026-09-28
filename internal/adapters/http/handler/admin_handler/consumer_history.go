package admin_handler

import (
	"context"
	"errors"
	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type consumerHistoryService interface {
	Query(context.Context, int, admin.ConsumerHistoryQuery, string, string) (*readmodel.ConsumerHistory, error)
}
type ConsumerHistoryHandler struct {
	service consumerHistoryService
	cursors *signedcursor.Codec
}

func NewConsumerHistoryHandler(service consumerHistoryService, key []byte) (*ConsumerHistoryHandler, error) {
	codec, err := signedcursor.New(key, consumerHistoryCursorPurpose)
	if err != nil {
		return nil, err
	}
	return &ConsumerHistoryHandler{service, codec}, nil
}
func (h *ConsumerHistoryHandler) Get(c *gin.Context) {
	subject, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlation, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, 500, "internal server error")
		return
	}
	id, err := parseConsumerHistoryID(c.Param("consumer_id"))
	if err != nil {
		httphandler.RespondError(c, 400, "invalid consumer ID")
		return
	}
	q, err := parseConsumerHistoryQuery(c.Request.URL.RawQuery, id, h.cursors)
	if err != nil {
		httphandler.RespondError(c, 400, "invalid consumer history query")
		return
	}
	model, err := h.service.Query(c.Request.Context(), id, q, subject, correlation)
	if errors.Is(err, admin.ErrConsumerHistoryNotFound) {
		httphandler.RespondError(c, 404, "consumer not found")
		return
	}
	if errors.Is(err, admin.ErrInvalidConsumerHistoryQuery) {
		httphandler.RespondError(c, 400, "invalid consumer history query")
		return
	}
	if err != nil {
		httphandler.RespondError(c, 500, "internal server error")
		return
	}
	response := consumerHistoryResponseFromModel(model, q.Limit)
	if model.HasMore && len(model.Items) > 0 {
		p := consumerHistoryCursor{Version: 1, ConsumerID: id, Type: q.Type, Status: q.Status, ProviderID: q.ProviderID, From: q.From, To: q.To, Limit: q.Limit, After: model.Items[len(model.Items)-1].Position()}
		token, err := h.cursors.Encode(p)
		if err != nil {
			httphandler.RespondError(c, 500, "internal server error")
			return
		}
		response.Page.NextCursor = &token
	}
	c.JSON(http.StatusOK, response)
}

type historyAddressResponse struct {
	Street       string  `json:"street"`
	StreetNumber string  `json:"street_number"`
	Floor        *string `json:"floor"`
	Unit         *string `json:"unit"`
	Source       string  `json:"source"`
}
type historyZoneResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Source  string `json:"source"`
}
type historyConsumerResponse struct {
	ID              int                     `json:"id"`
	Role            string                  `json:"role"`
	Name            string                  `json:"name"`
	Surname         string                  `json:"surname"`
	Email           string                  `json:"email"`
	ProfilePhotoURL *string                 `json:"profile_photo_url"`
	CreatedOn       time.Time               `json:"created_on"`
	Address         *historyAddressResponse `json:"address"`
	CoverageZone    *historyZoneResponse    `json:"coverage_zone"`
}
type historySummaryResponse struct {
	JobRequests      int `json:"job_requests"`
	ServiceProposals int `json:"service_proposals"`
	WorkOrders       int `json:"work_orders"`
}
type historyPartyResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}
type historyOperationResponse struct {
	ID                     string `json:"id"`
	URL                    string `json:"url"`
	RequiredPermission     string `json:"required_permission"`
	ChatRequiredPermission string `json:"chat_required_permission"`
}
type historyItemCommon struct {
	Type       string                   `json:"type"`
	ID         int                      `json:"id"`
	Status     string                   `json:"status"`
	Provider   historyPartyResponse     `json:"provider"`
	OccurredOn time.Time                `json:"occurred_on"`
	Operation  historyOperationResponse `json:"operation"`
}
type historyRequestResponse struct {
	historyItemCommon
	CreatedOn time.Time `json:"created_on"`
}
type historyProposalResponse struct {
	historyItemCommon
	JobRequestID             *int      `json:"job_request_id"`
	CreatedOn                time.Time `json:"created_on"`
	ScheduledOn              time.Time `json:"scheduled_on"`
	EstimatedDurationMinutes int       `json:"estimated_duration_minutes"`
	BookingPaymentDeadline   time.Time `json:"booking_payment_deadline"`
}
type historyOrderResponse struct {
	historyItemCommon
	JobRequestID         *int       `json:"job_request_id"`
	ServiceProposalID    int        `json:"service_proposal_id"`
	AcceptedOn           time.Time  `json:"accepted_on"`
	CompletionReportedOn *time.Time `json:"completion_reported_on"`
	BalancePaidOn        *time.Time `json:"balance_paid_on"`
}
type historyPageResponse struct {
	Items      []any   `json:"items"`
	Limit      int     `json:"limit"`
	NextCursor *string `json:"next_cursor"`
}
type consumerHistoryResponse struct {
	Consumer historyConsumerResponse `json:"consumer"`
	Summary  historySummaryResponse  `json:"summary"`
	Page     historyPageResponse     `json:"page"`
}

func consumerHistoryResponseFromModel(h *readmodel.ConsumerHistory, limit int) consumerHistoryResponse {
	c := h.Consumer
	r := consumerHistoryResponse{Consumer: historyConsumerResponse{ID: c.ID, Role: "consumer", Name: c.Name, Surname: c.Surname, Email: c.Email, CreatedOn: c.CreatedOn.UTC()}, Summary: historySummaryResponse{h.Summary.JobRequests, h.Summary.ServiceProposals, h.Summary.WorkOrders}, Page: historyPageResponse{Items: make([]any, 0, len(h.Items)), Limit: limit}}
	if c.ProfilePhotoURL != "" {
		r.Consumer.ProfilePhotoURL = &c.ProfilePhotoURL
	}
	if h.Address != nil {
		a := h.Address
		r.Consumer.Address = &historyAddressResponse{a.Street, a.StreetNumber, a.Floor, a.Unit, "current_consumer_profile"}
	}
	if h.CoverageZone != nil {
		z := h.CoverageZone
		r.Consumer.CoverageZone = &historyZoneResponse{z.ID, z.Name, z.Enabled, "current_consumer_profile"}
	}
	for _, i := range h.Items {
		operationID := string(i.Operation.Kind) + "-" + strconv.Itoa(i.Operation.ResourceID)
		common := historyItemCommon{i.Type, i.ID, i.Status, historyPartyResponse{i.Provider.ID, i.Provider.Name, i.Provider.Surname}, i.OccurredOn.UTC(), historyOperationResponse{operationID, "/admin/operations/" + operationID, "read:admin_operations", "read:admin_chat_audit"}}
		switch i.Type {
		case "job_request":
			r.Page.Items = append(r.Page.Items, historyRequestResponse{common, i.CreatedOn.UTC()})
		case "service_proposal":
			r.Page.Items = append(r.Page.Items, historyProposalResponse{common, i.JobRequestID, i.CreatedOn.UTC(), i.ScheduledOn.UTC(), i.EstimatedDurationMinutes, i.BookingPaymentDeadline.UTC()})
		case "work_order":
			r.Page.Items = append(r.Page.Items, historyOrderResponse{common, i.JobRequestID, i.ServiceProposalID, i.AcceptedOn.UTC(), optionalTimeUTC(i.CompletionReportedOn), optionalTimeUTC(i.BalancePaidOn)})
		}
	}
	return r
}
