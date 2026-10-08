package category_handler

import (
	"encoding/json"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
)

type editCategoryRequest struct{ Edit category.Edit }

func (request *editCategoryRequest) UnmarshalJSON(data []byte) error {
	fields, err := handler.StrictObject(data, "name", "enabled", "expected_version", "reason", "confirm_ongoing_orders")
	if err != nil {
		return err
	}
	for field, value := range fields {
		if string(value) == "null" {
			return handler.ErrInvalidJSONObject
		}
		switch field {
		case "name":
			request.Edit.Name = new(string)
			err = json.Unmarshal(value, request.Edit.Name)
		case "enabled":
			request.Edit.Enabled = new(bool)
			err = json.Unmarshal(value, request.Edit.Enabled)
		case "expected_version":
			err = json.Unmarshal(value, &request.Edit.ExpectedVersion)
		case "reason":
			err = json.Unmarshal(value, &request.Edit.Reason)
		case "confirm_ongoing_orders":
			err = json.Unmarshal(value, &request.Edit.ConfirmOngoingOrders)
		}
		if err != nil {
			return handler.ErrInvalidJSONObject
		}
	}
	return nil
}

type administrativeCategoryResponse struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Version int    `json:"version"`
}

func administrativeCategoryFromDomain(current category.Category) administrativeCategoryResponse {
	return administrativeCategoryResponse{ID: current.ID, Name: current.Name, Enabled: current.Enabled, Version: current.Version}
}

func (h *CategoryHandler) EditCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		handler.RespondError(c, http.StatusBadRequest, category.ErrIDRequired.Error())
		return
	}
	var req editCategoryRequest
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024))
	if err != nil {
		handler.RespondError(c, http.StatusBadRequest, errInvalidRequestBody.Error())
		return
	}
	if err := json.Unmarshal(data, &req); err != nil {
		handler.RespondError(c, http.StatusBadRequest, errInvalidRequestBody.Error())
		return
	}
	subject, ok := handler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlation, ok := middleware.GetRequestID(c)
	if !ok {
		handler.RespondError(c, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	edited, err := h.categoryService.EditCategory(c.Request.Context(), id, req.Edit, subject, correlation)
	if err != nil {
		handleCategoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, administrativeCategoryFromDomain(*edited))
}

type categoryImpactCountsResponse struct {
	AssignedProviders               int `json:"assigned_providers"`
	PendingRequests                 int `json:"pending_requests"`
	AcceptedRequestsWithoutProposal int `json:"accepted_requests_without_proposal"`
	PendingProposals                int `json:"pending_proposals"`
	ScheduledOrders                 int `json:"scheduled_orders"`
	AwaitingPaymentOrders           int `json:"awaiting_payment_orders"`
}

func (h *CategoryHandler) GetImpact(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		handler.RespondError(c, http.StatusBadRequest, category.ErrIDRequired.Error())
		return
	}
	impact, err := h.categoryService.GetImpact(c.Request.Context(), id)
	if err != nil {
		handleCategoryError(c, err)
		return
	}
	counts := impact.Counts
	c.JSON(http.StatusOK, gin.H{"category": administrativeCategoryFromDomain(impact.Category), "observed_at": impact.ObservedAt,
		"counts":             categoryImpactCountsResponse{counts.AssignedProviders, counts.PendingRequests, counts.AcceptedRequestsWithoutProposal, counts.PendingProposals, counts.ScheduledOrders, counts.AwaitingPaymentOrders},
		"has_ongoing_orders": impact.HasOngoingOrders(), "requires_confirmation": impact.RequiresConfirmation(), "existing_operations_can_continue": true})
}
