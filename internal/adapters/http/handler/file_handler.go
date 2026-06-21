package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileService *filedomain.Service
}

type presignFileRequest struct {
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int    `json:"size_bytes"`
	Purpose      string `json:"purpose"`
}

type presignFileResponse struct {
	FileID    string            `json:"file_id"`
	Key       string            `json:"key"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
}

type confirmFileRequest struct {
	Key       string `json:"key"`
	MimeType  string `json:"mime_type"`
	SizeBytes int    `json:"size_bytes"`
}

type fileResponse struct {
	ID           string `json:"id"`
	URL          string `json:"url,omitempty"`
	OriginalName string `json:"original_name"`
}

func NewFileHandler(fileService *filedomain.Service) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) PresignUpload(c *gin.Context) {
	authID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(authID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req presignFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.fileService.RequestUpload(c.Request.Context(), filedomain.PresignRequest{
		AuthID:       authID,
		OriginalName: strings.TrimSpace(req.OriginalName),
		MimeType:     strings.TrimSpace(strings.ToLower(req.MimeType)),
		SizeBytes:    req.SizeBytes,
		Purpose:      strings.TrimSpace(req.Purpose),
	})
	if err != nil {
		handleFileError(c, err)
		return
	}

	c.JSON(http.StatusOK, presignFileResponse{
		FileID:    result.FileID,
		Key:       result.Key,
		UploadURL: result.URL,
		Headers:   result.Headers,
	})
}

func (h *FileHandler) ConfirmUpload(c *gin.Context) {
	authID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(authID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req confirmFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	confirmedFile, err := h.fileService.ConfirmUpload(c.Request.Context(), filedomain.ConfirmRequest{
		AuthID:    authID,
		FileID:    strings.TrimSpace(c.Param("fileID")),
		Key:       strings.TrimSpace(req.Key),
		MimeType:  strings.TrimSpace(strings.ToLower(req.MimeType)),
		SizeBytes: req.SizeBytes,
	})
	if err != nil {
		handleFileError(c, err)
		return
	}

	c.JSON(http.StatusOK, fileResponse{
		ID:           confirmedFile.FileID,
		URL:          confirmedFile.URL,
		OriginalName: confirmedFile.OriginalName,
	})
}

func handleFileError(c *gin.Context, err error) {
	if errors.Is(err, filedomain.ErrUnsupportedProfilePhoto) || errors.Is(err, filedomain.ErrProfilePhotoNotAvailable) {
		c.JSON(http.StatusBadRequest, gin.H{"error": filedomain.ErrProfilePhotoNotAvailable.Error()})
		return
	}
	if errors.Is(err, filedomain.ErrMessageImageNotAvailable) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, filedomain.ErrFileNotAvailable) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, filedomain.ErrProfilePhotoRequired) ||
		errors.Is(err, filedomain.ErrOriginalNameRequired) ||
		errors.Is(err, filedomain.ErrMimeTypeRequired) ||
		errors.Is(err, filedomain.ErrSizeRequired) ||
		errors.Is(err, filedomain.ErrPurposeRequired) ||
		errors.Is(err, filedomain.ErrUnsupportedPurpose) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
