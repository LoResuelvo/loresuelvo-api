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
