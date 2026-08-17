package work_order_handler

import (
	"context"
	"fmt"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
	"github.com/gin-gonic/gin"
)

type WorkOrderHandler struct {
	workOrderService workOrderService
}

type workOrderService interface {
	GetWorkOrders(ctx context.Context, auth0ID string) ([]readmodel.WorkOrderSummary, error)
	GetWorkOrder(ctx context.Context, auth0ID string, workOrderID int) (*readmodel.WorkOrderDetail, error)
	ReportCompletion(ctx context.Context, auth0ID string, workOrderID int, description string, imageFileIDs []string) (*readmodel.CompletionReport, error)
	CreateReview(ctx context.Context, auth0ID string, workOrderID int, rating int, description string) (*readmodel.Review, error)
}

func NewWorkOrderHandler(workOrderService workOrderService) *WorkOrderHandler {
	return &WorkOrderHandler{workOrderService: workOrderService}
}

func (h *WorkOrderHandler) GetWorkOrders(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	orders, err := h.workOrderService.GetWorkOrders(c.Request.Context(), auth0ID)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "Could not get work orders")
		return
	}

	c.JSON(http.StatusOK, workOrderSummaryResponsesFromReadModel(orders))
}

func (h *WorkOrderHandler) GetWorkOrder(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	workOrderID, err := httphandler.PositiveIDFromString(c.Param("workOrderID"), "work order id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	detail, err := h.workOrderService.GetWorkOrder(c.Request.Context(), auth0ID, workOrderID)
	if err != nil {
		handleGetWorkOrderError(c, err)
		return
	}

	c.JSON(http.StatusOK, workOrderDetailResponseFromReadModel(*detail))
}

func (h *WorkOrderHandler) ReportCompletion(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	workOrderID, err := httphandler.PositiveIDFromString(c.Param("workOrderID"), "work order id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var req reportCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.workOrderService.ReportCompletion(
		c.Request.Context(),
		auth0ID,
		workOrderID,
		req.Description,
		req.ImageFileIDs,
	)
	if err != nil {
		handleReportCompletionError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/work-orders/%d", workOrderID))
	c.JSON(http.StatusCreated, completionReportResponseFromReadModel(*report))
}

func (h *WorkOrderHandler) CreateReview(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	workOrderID, err := httphandler.PositiveIDFromString(c.Param("workOrderID"), "work order id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var req createReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	review, err := h.workOrderService.CreateReview(
		c.Request.Context(),
		auth0ID,
		workOrderID,
		req.Rating,
		req.Description,
	)
	if err != nil {
		handleCreateReviewError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/work-orders/%d", workOrderID))
	c.JSON(http.StatusCreated, reviewResponseFromReadModel(*review))
}
