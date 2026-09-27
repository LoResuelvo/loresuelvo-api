package operation_media_handler

import (
	"context"
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/gin-gonic/gin"
)

type service interface {
	Get(ctx context.Context, operationID, fileID string) (*operation.OperationImage, error)
}

type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

func (handler *Handler) Get(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	image, err := handler.service.Get(c.Request.Context(), c.Param("operation_id"), c.Param("file_id"))
	if errors.Is(err, operation.ErrInvalidOperationID) || errors.Is(err, operation.ErrInvalidOperationImageID) {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid operation or image ID")
		return
	}
	if errors.Is(err, operation.ErrOperationImageNotFound) {
		httphandler.RespondError(c, http.StatusNotFound, "operation image not found")
		return
	}
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.Data(http.StatusOK, image.MimeType, image.Bytes)
}
