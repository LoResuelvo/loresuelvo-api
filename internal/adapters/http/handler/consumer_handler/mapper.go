package consumer_handler

import (
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

func normalizeRegisterConsumerRequest(req registerConsumerRequest) registerConsumerRequest {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)
	req.ProfilePhotoFileID = strings.TrimSpace(req.ProfilePhotoFileID)
	if req.Address != nil {
		req.Address.Street = strings.TrimSpace(req.Address.Street)
		req.Address.StreetNumber = strings.TrimSpace(req.Address.StreetNumber)
		req.Address.Floor = strings.TrimSpace(req.Address.Floor)
		req.Address.Unit = strings.TrimSpace(req.Address.Unit)
	}

	return req
}

func consumerSummaryResponseFromDomain(consumer consumer.Consumer) consumerSummaryResponse {
	profilePhotoURL := ""
	if consumer.ProfilePhoto() != nil {
		profilePhotoURL = consumer.ProfilePhoto().URL
	}
	return consumerSummaryResponse{
		ID:              consumer.ID(),
		Name:            consumer.Name(),
		Surname:         consumer.Surname(),
		ProfilePhotoURL: profilePhotoURL,
	}
}
