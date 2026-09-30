package admin_payment_handler

import (
	"context"
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/gin-gonic/gin"
)

type queryService interface {
	Query(context.Context, payment.AdminPaymentQuery, string, string) (*readmodel.AdminPaymentPage, error)
}

type Handler struct {
	service queryService
	cursors *signedcursor.Codec
}

func NewHandler(service queryService, signingKey []byte) (*Handler, error) {
	codec, err := signedcursor.New(signingKey, cursorPurpose)
	if err != nil {
		return nil, err
	}
	return &Handler{service: service, cursors: codec}, nil
}

func (handler *Handler) List(c *gin.Context) {
	subject, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlation, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	query, err := parseQuery(c.Request.URL.RawQuery, handler.cursors)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid administrative payment query")
		return
	}
	page, err := handler.service.Query(c.Request.Context(), query, subject, correlation)
	if errors.Is(err, payment.ErrInvalidAdminPaymentQuery) {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid administrative payment query")
		return
	}
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	response, err := responseFromPage(page, handler.cursors, query)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, response)
}
