package job_request_handler

import (
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request/read_model"
)

func jobRequestResponseFromDomain(createdJobRequest jobrequest.JobRequest) jobRequestResponse {
	return jobRequestResponse{
		ID:             createdJobRequest.ID,
		ConversationID: createdJobRequest.ConversationID,
		Title:          createdJobRequest.Title,
		Description:    createdJobRequest.Description,
		Status:         string(createdJobRequest.Status),
		Images:         jobRequestImageResponsesFromDomain(createdJobRequest.Images),
	}
}

func jobRequestSummaryResponsesFromReadModel(jobRequests []readmodel.JobRequestSummary) []jobRequestSummaryResponse {
	formattedJobRequests := make([]jobRequestSummaryResponse, len(jobRequests))
	for i, jobRequest := range jobRequests {
		formattedJobRequests[i] = jobRequestSummaryResponse{
			ID:             jobRequest.ID,
			ConversationID: jobRequest.ConversationID,
			Title:          jobRequest.Title,
			Description:    jobRequest.Description,
			Status:         jobRequest.Status,
			Requester: jobRequestRequesterResponse{
				Name:    jobRequest.Requester.Name,
				Surname: jobRequest.Requester.Surname,
			},
			Images: jobRequestImageResponsesFromReadModel(jobRequest.Images),
		}
	}

	return formattedJobRequests
}

func jobRequestImageResponsesFromDomain(images []filedomain.MessageImage) []messageImageResponse {
	response := make([]messageImageResponse, 0, len(images))
	for _, image := range images {
		response = append(response, messageImageResponse{
			ID:           image.FileID,
			URL:          image.URL,
			OriginalName: image.OriginalName,
		})
	}
	return response
}

func jobRequestImageResponsesFromReadModel(images []readmodel.JobRequestImage) []messageImageResponse {
	response := make([]messageImageResponse, 0, len(images))
	for _, image := range images {
		response = append(response, messageImageResponse{
			ID:           image.FileID,
			URL:          image.URL,
			OriginalName: image.OriginalName,
		})
	}
	return response
}
