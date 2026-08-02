package work_order_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
)

type WorkOrderHandler struct {
	workOrderService *workorder.Service
}

func NewWorkOrderHandler(workOrderService *workorder.Service) *WorkOrderHandler {
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

func (h *WorkOrderHandler) GetConfirmationCode(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	workOrderID, err := httphandler.PositiveIDFromString(c.Param("workOrderID"), "work order id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	code, err := h.workOrderService.GetConfirmationCode(c.Request.Context(), auth0ID, workOrderID)
	if err != nil {
		handleGetConfirmationCodeError(c, err)
		return
	}
	c.JSON(http.StatusOK, confirmationCodeResponse{ConfirmationCode: code.String()})
}

func handleGetConfirmationCodeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, workorder.ErrDoesNotExist):
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, workorder.ErrOnlyConsumerCanViewConfirmationCode):
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
	case errors.Is(err, workorder.ErrConfirmationCodeNotAvailable):
		httphandler.RespondError(c, http.StatusConflict, err.Error())
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "Could not get work order confirmation code")
	}
}
