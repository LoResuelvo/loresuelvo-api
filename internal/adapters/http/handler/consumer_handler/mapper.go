package consumer_handler

import "strings"

func normalizeRegisterConsumerRequest(req registerConsumerRequest) registerConsumerRequest {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)

	return req
}

func registeredConsumerResponse() messageResponse {
	return messageResponse{Message: "cuenta registrada exitosamente"}
}
