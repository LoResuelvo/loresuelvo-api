package file_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileService *filedomain.Service
}

func NewFileHandler(fileService *filedomain.Service) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) PresignUpload(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req presignFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.fileService.RequestUpload(c.Request.Context(), presignRequestFromHTTP(authID, req))
	if err != nil {
		handleFileError(c, err)
		return
	}

	c.JSON(http.StatusOK, presignFileResponseFromDomain(result))
}

func (h *FileHandler) ConfirmUpload(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req confirmFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	confirmedFile, err := h.fileService.ConfirmUpload(c.Request.Context(), confirmRequestFromHTTP(authID, c.Param("fileID"), req))
	if err != nil {
		handleFileError(c, err)
		return
	}

	c.JSON(http.StatusOK, fileResponseFromDomain(confirmedFile))
}
