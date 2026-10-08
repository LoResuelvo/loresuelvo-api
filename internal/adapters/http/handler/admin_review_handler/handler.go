package admin_review_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type service interface {
	List(context.Context, workorder.ReviewListInput) (*workorder.AdminReviewPage, error)
	Get(context.Context, string, int, workorder.ReviewPageInput, string) (*workorder.AdminReviewDetail, error)
	Moderate(context.Context, string, int, workorder.ModerationInput, string) (*workorder.ReviewModerationResult, error)
}
type Handler struct{ service service }

func New(s service) *Handler { return &Handler{service: s} }
func respondError(c *gin.Context, err error) {
	status := 500
	message := "internal server error"
	switch {
	case errors.Is(err, workorder.ErrInvalidReviewModeration):
		status = 400
		message = workorder.ErrInvalidReviewModeration.Error()
	case errors.Is(err, workorder.ErrReviewReportForbidden):
		status = 403
		message = "forbidden"
	case errors.Is(err, workorder.ErrReviewNotAvailable):
		status = 404
		message = workorder.ErrReviewNotAvailable.Error()
	case errors.Is(err, workorder.ErrReviewModerationConflict):
		status = 409
		message = workorder.ErrReviewModerationConflict.Error()
	}
	httphandler.RespondError(c, status, message)
}
func pageInput(values url.Values, allowedStatus bool) (workorder.ReviewPageInput, error) {
	var page workorder.ReviewPageInput
	for name, entries := range values {
		if len(entries) != 1 {
			return page, workorder.ErrInvalidReviewModeration
		}
		switch name {
		case "page", "decisions_page":
			if (name == "page") != allowedStatus {
				return page, workorder.ErrInvalidReviewModeration
			}
			n, err := strconv.Atoi(entries[0])
			if err != nil || n <= 0 {
				return page, workorder.ErrInvalidReviewModeration
			}
			page.Page = n
		case "limit", "decisions_limit":
			if (name == "limit") != allowedStatus {
				return page, workorder.ErrInvalidReviewModeration
			}
			n, err := strconv.Atoi(entries[0])
			if err != nil || n <= 0 {
				return page, workorder.ErrInvalidReviewModeration
			}
			page.Limit = n
		case "status":
			if !allowedStatus {
				return page, workorder.ErrInvalidReviewModeration
			}
		default:
			return page, workorder.ErrInvalidReviewModeration
		}
	}
	return page.Normalize()
}
func (h *Handler) List(c *gin.Context) {
	if _, ok := httphandler.GetAuthenticatedUserID(c); !ok {
		return
	}
	page, err := pageInput(c.Request.URL.Query(), true)
	if err != nil {
		respondError(c, err)
		return
	}
	result, err := h.service.List(c.Request.Context(), workorder.ReviewListInput{ReviewPageInput: page, Status: c.Query("status")})
	if err != nil {
		respondError(c, err)
		return
	}
	items := make([]summaryResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, summary(item))
	}
	c.JSON(200, gin.H{"items": items, "page": result.Page, "limit": result.Limit, "total": result.Total})
}
func (h *Handler) Get(c *gin.Context) {
	auth, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	id, err := httphandler.PositiveIDFromString(c.Param("id"), "review id")
	if err != nil {
		respondError(c, workorder.ErrInvalidReviewModeration)
		return
	}
	page, err := pageInput(c.Request.URL.Query(), false)
	if err != nil {
		respondError(c, err)
		return
	}
	correlation, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, 500, "internal server error")
		return
	}
	detail, err := h.service.Get(c.Request.Context(), auth, id, page, correlation)
	if err != nil {
		respondError(c, err)
		return
	}
	decisions := make([]decisionResponse, 0, len(detail.Decisions))
	for _, d := range detail.Decisions {
		decisions = append(decisions, decision(d))
	}
	c.JSON(200, gin.H{"review": summary(detail.Summary), "description": detail.Review.Description(), "report": report(detail.Report), "decisions": gin.H{"items": decisions, "page": detail.Page, "limit": detail.Limit, "total": detail.Total, "has_more": int64(detail.Page*detail.Limit) < detail.Total}})
}
func (h *Handler) Moderate(c *gin.Context) {
	auth, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	id, err := httphandler.PositiveIDFromString(c.Param("id"), "review id")
	if err != nil {
		respondError(c, workorder.ErrInvalidReviewModeration)
		return
	}
	input, err := decodeModeration(c)
	if err != nil {
		respondError(c, err)
		return
	}
	correlation, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, 500, "internal server error")
		return
	}
	result, err := h.service.Moderate(c.Request.Context(), auth, id, input, correlation)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"work_order_id": id, "visibility": visibility(result.Review.Visible()), "version": result.Review.Version(), "decision_id": result.Decision.ID()})
}
func decodeModeration(c *gin.Context) (workorder.ModerationInput, error) {
	var input workorder.ModerationInput
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024))
	if err != nil {
		return input, workorder.ErrInvalidReviewModeration
	}
	fields, err := httphandler.StrictObject(data, "action", "category", "reason", "expected_version", "report_id")
	if err != nil {
		return input, workorder.ErrInvalidReviewModeration
	}
	for name, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return input, workorder.ErrInvalidReviewModeration
		}
		switch name {
		case "action":
			err = json.Unmarshal(value, &input.Action)
		case "category":
			err = json.Unmarshal(value, &input.Category)
		case "reason":
			err = json.Unmarshal(value, &input.Reason)
		case "expected_version":
			err = json.Unmarshal(value, &input.ExpectedVersion)
		case "report_id":
			err = json.Unmarshal(value, &input.ReportID)
			if input.ReportID <= 0 {
				return input, workorder.ErrInvalidReviewModeration
			}
		}
		if err != nil {
			return input, workorder.ErrInvalidReviewModeration
		}
	}
	return input.Normalize()
}
func visibility(visible bool) string {
	if visible {
		return "visible"
	}
	return "hidden"
}

type summaryResponse struct {
	WorkOrderID    int        `json:"work_order_id"`
	ConsumerID     int        `json:"consumer_id"`
	ProviderID     int        `json:"provider_id"`
	OperationID    string     `json:"operation_id"`
	Rating         int        `json:"rating"`
	Visibility     string     `json:"visibility"`
	Version        int        `json:"version"`
	PendingReports int        `json:"pending_report_count"`
	ReportedOn     *time.Time `json:"reported_on"`
}

func summary(s readmodel.AdminReviewSummary) summaryResponse {
	return summaryResponse{s.WorkOrderID, s.ConsumerID, s.ProviderID, s.OperationID, s.Rating, visibility(s.Visible), s.Version, s.PendingReports, s.ReportedOn}
}

type decisionResponse struct {
	ID             int       `json:"id"`
	WorkOrderID    int       `json:"work_order_id"`
	OperatorID     int       `json:"operator_id"`
	Action         string    `json:"action"`
	Category       *string   `json:"category"`
	Reason         string    `json:"reason"`
	ReportID       *int      `json:"report_id"`
	PreviousHideID *int      `json:"previous_hide_id"`
	CreatedOn      time.Time `json:"created_on"`
}

func decision(d *workorder.ReviewDecision) decisionResponse {
	result := decisionResponse{ID: d.ID(), WorkOrderID: d.WorkOrderID(), OperatorID: d.OperatorID(), Action: d.Action(), Reason: d.Reason(), CreatedOn: d.CreatedOn()}
	if d.Category() != "" {
		x := d.Category()
		result.Category = &x
	}
	if d.ReportID() > 0 {
		x := d.ReportID()
		result.ReportID = &x
	}
	if d.PreviousHideID() > 0 {
		x := d.PreviousHideID()
		result.PreviousHideID = &x
	}
	return result
}

type reportResponse struct {
	ID          int       `json:"id"`
	Category    string    `json:"category"`
	Explanation string    `json:"explanation"`
	Status      string    `json:"status"`
	CreatedOn   time.Time `json:"created_on"`
}

func report(r *workorder.ReviewReport) *reportResponse {
	if r == nil {
		return nil
	}
	return &reportResponse{ID: r.ID(), Category: r.Category(), Explanation: r.Explanation(), Status: r.Status(), CreatedOn: r.CreatedOn()}
}
