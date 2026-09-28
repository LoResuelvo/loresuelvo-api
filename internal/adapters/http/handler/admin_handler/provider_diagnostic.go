package admin_handler

import (
	"context"
	"errors"
	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type diagnosticService interface {
	Query(ctx context.Context, providerID, subject, correlationID string) (*readmodel.ProviderDiagnostic, error)
}
type ProviderDiagnosticHandler struct{ service diagnosticService }

func NewProviderDiagnosticHandler(service diagnosticService) *ProviderDiagnosticHandler {
	return &ProviderDiagnosticHandler{service}
}
func (h *ProviderDiagnosticHandler) Get(c *gin.Context) {
	subject, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlation, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, 500, "internal server error")
		return
	}
	d, err := h.service.Query(c.Request.Context(), c.Param("provider_id"), subject, correlation)
	if errors.Is(err, admin.ErrInvalidDiagnosticProviderID) {
		httphandler.RespondError(c, 400, "invalid provider ID")
		return
	}
	if errors.Is(err, admin.ErrProviderDiagnosticNotFound) {
		httphandler.RespondError(c, 404, "provider not found")
		return
	}
	if err != nil {
		httphandler.RespondError(c, 500, "internal server error")
		return
	}
	c.JSON(http.StatusOK, diagnosticResponseFromModel(d))
}

type diagnosticCheckResponse struct {
	Control    string     `json:"control"`
	Result     string     `json:"result"`
	ReasonCode string     `json:"reason_code"`
	EvidenceOn *time.Time `json:"evidence_on"`
}
type diagnosticPaymentResponse struct {
	State          string     `json:"state"`
	TokenExpiresOn *time.Time `json:"token_expires_on"`
}
type diagnosticCalendarResponse struct {
	State     string                        `json:"calendar_connection_status"`
	OrderSync []diagnosticOrderSyncResponse `json:"order_sync"`
}
type diagnosticActivityResponse struct {
	Type       string    `json:"type"`
	ID         int       `json:"id"`
	Status     string    `json:"status"`
	OccurredOn time.Time `json:"occurred_on"`
}
type diagnosticReviewResponse struct {
	WorkOrderID int    `json:"work_order_id"`
	Rating      int    `json:"rating"`
	Description string `json:"description"`
}
type diagnosticOrderSyncResponse struct {
	WorkOrderID int        `json:"work_order_id"`
	SyncedOn    *time.Time `json:"synced_on"`
}
type diagnosticReputationResponse struct {
	Count   int     `json:"count"`
	Average float64 `json:"average"`
}
type diagnosticNavigationResponse struct {
	OperationsURL      string `json:"operations_url"`
	RequiredPermission string `json:"required_permission"`
}
type providerDiagnosticResponse struct {
	Activity   []diagnosticActivityResponse `json:"activity"`
	Reviews    []diagnosticReviewResponse   `json:"reviews"`
	Reputation diagnosticReputationResponse `json:"reputation"`
	Navigation diagnosticNavigationResponse `json:"navigation"`
	Provider   providerDirectoryResponse    `json:"provider"`
	Checks     []diagnosticCheckResponse    `json:"checks"`
	Payment    diagnosticPaymentResponse    `json:"payment"`
	Calendar   diagnosticCalendarResponse   `json:"calendar"`
}

func diagnosticResponseFromModel(d *readmodel.ProviderDiagnostic) providerDiagnosticResponse {
	checks := make([]diagnosticCheckResponse, 0, len(d.DiagnosticChecks))
	for _, check := range d.DiagnosticChecks {
		checks = append(checks, diagnosticCheckResponse{check.Control, check.Result, check.ReasonCode, optionalTimeUTC(check.EvidenceOn)})
	}

	activity := make([]diagnosticActivityResponse, 0, len(d.Activity))
	for _, a := range d.Activity {
		activity = append(activity, diagnosticActivityResponse{a.Type, a.ID, a.Status, a.OccurredOn.UTC()})
	}
	reviews := make([]diagnosticReviewResponse, 0, len(d.Reviews))
	for _, r := range d.Reviews {
		reviews = append(reviews, diagnosticReviewResponse{r.WorkOrderID, r.Rating, r.Description})
	}
	orderSync := make([]diagnosticOrderSyncResponse, 0, len(d.OrderSync))
	for _, e := range d.OrderSync {
		orderSync = append(orderSync, diagnosticOrderSyncResponse{e.WorkOrderID, optionalTimeUTC(e.SyncedOn)})
	}
	reputation := d.Reputation()
	return providerDiagnosticResponse{
		Provider: providerDirectoryResponsesFromReadModel([]readmodel.Provider{d.Provider})[0], Checks: checks,
		Payment: diagnosticPaymentResponse{d.Payment.State(), optionalTimeUTC(d.Payment.TokenExpiresOn)}, Calendar: diagnosticCalendarResponse{d.Calendar.State(), orderSync},
		Activity: activity, Reviews: reviews, Reputation: diagnosticReputationResponse{reputation.Count, reputation.Average}, Navigation: diagnosticNavigationResponse{"/admin/operations?provider_id=" + strconv.Itoa(d.Provider.ID), "read:admin_operations"},
	}
}
