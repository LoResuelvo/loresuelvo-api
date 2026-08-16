package work_order_handler

import readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"

func workOrderSummaryResponsesFromReadModel(summaries []readmodel.WorkOrderSummary) []workOrderSummaryResponse {
	responses := make([]workOrderSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		responses = append(responses, workOrderSummaryResponse{
			ID:                summary.ID,
			ServiceProposalID: summary.ServiceProposalID,
			AmountCents:       summary.Amount,
			ScheduledOn:       summary.ScheduledOn,
			Description:       summary.Description,
			Status:            summary.Status,
			AcceptedOn:        summary.AcceptedOn,
			Counterpart: workOrderCounterpartResponse{
				ID:              summary.Counterpart.ID,
				Role:            summary.Counterpart.Role,
				Name:            summary.Counterpart.Name,
				Surname:         summary.Counterpart.Surname,
				CategoryName:    summary.Counterpart.CategoryName,
				ProfilePhotoURL: summary.Counterpart.ProfilePhotoURL,
			},
		})
	}
	return responses
}

func completionReportResponseFromReadModel(report readmodel.CompletionReport) completionReportResponse {
	images := make([]completionImageResponse, 0, len(report.Images))
	for _, image := range report.Images {
		images = append(images, completionImageResponse{
			FileID:       image.FileID,
			OriginalName: image.OriginalName,
			URL:          image.URL,
		})
	}

	return completionReportResponse{
		ID:          report.ID,
		Description: report.Description,
		ReportedOn:  report.ReportedOn,
		Images:      images,
	}
}

func workOrderDetailResponseFromReadModel(detail readmodel.WorkOrderDetail) workOrderDetailResponse {
	response := workOrderDetailResponse{
		ID:                detail.ID,
		ServiceProposalID: detail.ServiceProposalID,
		ConsumerID:        detail.ConsumerID,
		ProviderID:        detail.ProviderID,
		AmountCents:       detail.Amount,
		ScheduledOn:       detail.ScheduledOn,
		Description:       detail.Description,
		Status:            detail.Status,
		AcceptedOn:        detail.AcceptedOn,
	}
	if !detail.PaidOn.IsZero() {
		paidOn := detail.PaidOn
		response.PaidOn = &paidOn
	}
	if detail.CompletionReport != nil {
		report := completionReportResponseFromReadModel(*detail.CompletionReport)
		response.CompletionReport = &report
	}
	return response
}
