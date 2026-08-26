package consumer_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/gin-gonic/gin"
)

type ConsumerHandler struct {
	consumerService *consumer.Service
}

func NewConsumerHandler(consumerService *consumer.Service) *ConsumerHandler {
	return &ConsumerHandler{
		consumerService: consumerService,
	}
}

func (h *ConsumerHandler) RegisterConsumer(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req registerConsumerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	req = normalizeRegisterConsumerRequest(req)
	address := consumer.Address{}
	if req.Address != nil {
		address = consumer.Address{
			Street:       req.Address.Street,
			StreetNumber: req.Address.StreetNumber,
			Floor:        req.Address.Floor,
			Unit:         req.Address.Unit,
		}
	}

	createdConsumer, err := h.consumerService.RegisterConsumer(c.Request.Context(), auth0ID, req.Email, req.Name, req.Surname, req.ProfilePhotoFileID, address)
	if err != nil {
		handleRegisterConsumerError(c, err)
		return
	}

	c.JSON(http.StatusCreated, consumerSummaryResponseFromDomain(*createdConsumer))
}
