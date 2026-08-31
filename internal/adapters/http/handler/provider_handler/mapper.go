package provider_handler

import (
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

func normalizeRegisterProviderRequest(req registerProviderRequest) registerProviderRequest {
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.Surname = strings.TrimSpace(req.Surname)
	req.ProfilePhotoFileID = strings.TrimSpace(req.ProfilePhotoFileID)

	return req
}

func providerSummaryResponseFromDomain(provider provider.Provider) providerSummaryResponse {
	profilePhotoURL := ""
	if provider.ProfilePhoto() != nil {
		profilePhotoURL = provider.ProfilePhoto().URL
	}
	return providerSummaryResponse{
		ID:              provider.ID(),
		Name:            provider.Name(),
		Surname:         provider.Surname(),
		CategoryName:    provider.Categoryname(),
		ProfilePhotoURL: profilePhotoURL,
	}
}

func providerSearchResponsesFromReadModel(results []readmodel.ProviderSearchResult) []providerSearchResponse {
	response := make([]providerSearchResponse, 0, len(results))
	for _, result := range results {
		profilePhotoURL := ""
		if result.ProfilePhoto != nil {
			profilePhotoURL = result.ProfilePhoto.URL
		}
		response = append(response, providerSearchResponse{
			ID:               result.ID,
			Name:             result.Name,
			Surname:          result.Surname,
			CategoryName:     result.CategoryName,
			ProfilePhotoURL:  profilePhotoURL,
			RatingAverage:    result.RatingAverage,
			RatingCount:      result.RatingCount,
			IdentityVerified: result.IdentityVerified,
		})
	}

	return response
}

func providerProfileResponseFromReadModel(profile readmodel.Profile) providerProfileResponse {
	workOrders := make([]providerProfileWorkOrderResponse, 0, len(profile.WorkOrders))
	for _, workOrder := range profile.WorkOrders {
		workOrderResponse := providerProfileWorkOrderResponse{
			ID:          workOrder.ID,
			ScheduledOn: workOrder.ScheduledOn,
			Description: workOrder.Description,
			Status:      workOrder.Status,
		}
		if workOrder.CompletionReport != nil {
			workOrderResponse.CompletionReport = &providerProfileCompletionReportResponse{
				Description: workOrder.CompletionReport.Description,
				ReportedOn:  workOrder.CompletionReport.ReportedOn,
			}
		}
		if workOrder.Review != nil {
			workOrderResponse.Review = &providerProfileReviewResponse{
				Rating:      workOrder.Review.Rating,
				Description: workOrder.Review.Description,
			}
		}
		workOrders = append(workOrders, workOrderResponse)
	}

	profilePhoto := providerProfilePhotoResponse{}
	if profile.ProfilePhoto != nil {
		profilePhoto = providerProfilePhotoResponse{
			OriginalName: profile.ProfilePhoto.OriginalName,
			URL:          profile.ProfilePhoto.URL,
		}
	}

	return providerProfileResponse{
		ID:            profile.ID,
		Name:          profile.Name,
		Surname:       profile.Surname,
		ProfilePhoto:  profilePhoto,
		Category:      providerProfileCategoryResponse{ID: profile.CategoryID, Name: profile.CategoryName},
		RatingAverage: profile.RatingAverage,
		RatingCount:   profile.RatingCount,
		WorkOrders:    workOrders,
	}
}
