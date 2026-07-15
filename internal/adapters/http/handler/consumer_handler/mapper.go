package consumer_handler

import "strings"

import readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer/read_model"

func normalizeRegisterConsumerRequest(req registerConsumerRequest) registerConsumerRequest {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)
	req.ProfilePhotoFileID = strings.TrimSpace(req.ProfilePhotoFileID)

	return req
}

func consumerSummaryResponseFromDomain(consumer readmodel.ConsumerSummary) consumerSummaryResponse {
	return consumerSummaryResponse{
		ID:              consumer.ID,
		Name:            consumer.Name,
		Surname:         consumer.Surname,
		ProfilePhotoURL: consumer.ProfilePhotoURL,
	}
}
