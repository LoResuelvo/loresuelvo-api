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
	State string `json:"state"`
}
type providerDiagnosticResponse struct {
	Provider providerDirectoryResponse  `json:"provider"`
	Checks   []diagnosticCheckResponse  `json:"checks"`
	Payment  diagnosticPaymentResponse  `json:"payment"`
	Calendar diagnosticCalendarResponse `json:"calendar"`
}

func diagnosticResponseFromModel(d *readmodel.ProviderDiagnostic) providerDiagnosticResponse {
	checks := make([]diagnosticCheckResponse, 0, len(d.DiagnosticChecks))
	for _, check := range d.DiagnosticChecks {
		checks = append(checks, diagnosticCheckResponse{check.Control, check.Result, check.ReasonCode, optionalTimeUTC(check.EvidenceOn)})
	}
	return providerDiagnosticResponse{Provider: providerDirectoryResponsesFromReadModel([]readmodel.Provider{d.Provider})[0], Checks: checks, Payment: diagnosticPaymentResponse{d.Payment.State(), optionalTimeUTC(d.Payment.TokenExpiresOn)}, Calendar: diagnosticCalendarResponse{d.Calendar.State()}}
}
