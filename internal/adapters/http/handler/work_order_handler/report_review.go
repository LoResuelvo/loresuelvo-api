package work_order_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
)

type reviewReporter interface {
	Report(context.Context, string, int, string, string) (*workorder.ReviewReport, error)
}

type ReviewReportHandler struct{ service reviewReporter }

type reviewReportRequest struct{ category, explanation string }
type reviewReportResponse struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

func NewReviewReportHandler(service reviewReporter) *ReviewReportHandler {
	return &ReviewReportHandler{service: service}
}

func (h *ReviewReportHandler) Report(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	id, err := httphandler.PositiveIDFromString(c.Param("workOrderID"), "work order id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid work order id")
		return
	}
	request, err := decodeReviewReport(c)
	if err != nil {
		handleReviewReportError(c, err)
		return
	}
	report, err := h.service.Report(c.Request.Context(), authID, id, request.category, request.explanation)
	if err != nil {
		handleReviewReportError(c, err)
		return
	}
	c.JSON(http.StatusCreated, reviewReportResponse{ID: report.ID(), Status: report.Status()})
}

func decodeReviewReport(c *gin.Context) (reviewReportRequest, error) {
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024))
	if err != nil {
		return reviewReportRequest{}, workorder.ErrInvalidReviewReport
	}
	fields, err := httphandler.StrictObject(data, "category", "explanation")
	if err != nil {
		return reviewReportRequest{}, workorder.ErrInvalidReviewReport
	}
	var request reviewReportRequest
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return reviewReportRequest{}, workorder.ErrInvalidReviewReport
		}
		switch key {
		case "category":
			err = json.Unmarshal(value, &request.category)
		case "explanation":
			err = json.Unmarshal(value, &request.explanation)
		}
		if err != nil {
			return reviewReportRequest{}, workorder.ErrInvalidReviewReport
		}
	}
	return request, nil
}

func handleReviewReportError(c *gin.Context, err error) {
	status, message := http.StatusInternalServerError, "internal server error"
	switch {
	case errors.Is(err, workorder.ErrInvalidReviewReport):
		status, message = http.StatusBadRequest, workorder.ErrInvalidReviewReport.Error()
	case errors.Is(err, workorder.ErrReviewReportForbidden):
		status, message = http.StatusForbidden, workorder.ErrReviewReportForbidden.Error()
	case errors.Is(err, workorder.ErrReviewNotAvailable):
		status, message = http.StatusNotFound, workorder.ErrReviewNotAvailable.Error()
	case errors.Is(err, workorder.ErrReviewReportAlreadyExists):
		status, message = http.StatusConflict, workorder.ErrReviewReportAlreadyExists.Error()
	}
	httphandler.RespondError(c, status, message)
}
