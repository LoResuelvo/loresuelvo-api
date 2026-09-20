package admin_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	directoryService *admin.DirectoryService
}

func NewAdminHandler(directoryService *admin.DirectoryService) *AdminHandler {
	return &AdminHandler{directoryService: directoryService}
}

func (handler *AdminHandler) ListConsumers(c *gin.Context) {
	consumers, err := handler.directoryService.ListConsumers(c.Request.Context())
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, consumerDirectoryResponsesFromReadModel(consumers))
}
