package work_order_handler

import (
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
