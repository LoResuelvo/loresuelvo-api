package user_handler

import (
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

func currentUserResponseFromDomain(currentUser user.User, calendarConnectionStatus string) (any, error) {
	baseResponse := baseCurrentUserResponseFromDomain(currentUser, calendarConnectionStatus)

	switch typedUser := currentUser.(type) {
	case *consumer.Consumer:
		address := typedUser.Address()
		location := typedUser.Location()
		coverageZone := typedUser.CoverageZone()
		if coverageZone.ID <= 0 {
			return nil, fmt.Errorf("mapping consumer current user: coverage zone is missing")
		}
		return consumerCurrentUserResponse{
			currentUserResponse: baseResponse,
			Address: consumerAddressResponse{
				Street:         address.Street,
				StreetNumber:   address.StreetNumber,
				Floor:          address.Floor,
				Unit:           address.Unit,
				Latitude:       location.Latitude,
				Longitude:      location.Longitude,
				CoverageZoneID: coverageZone.ID,
			},
		}, nil
	case *provider.Provider:
		return providerCurrentUserResponse{
			currentUserResponse: baseResponse,
			Category: currentUserCategoryResponse{
				ID:   typedUser.Category.ID,
				Name: typedUser.Category.Name,
			},
		}, nil
	default:
		return nil, fmt.Errorf("mapping unsupported current user type %T", currentUser)
	}
}

func baseCurrentUserResponseFromDomain(currentUser user.User, calendarConnectionStatus string) currentUserResponse {
	response := currentUserResponse{
		ID:                       currentUser.ID(),
		Name:                     currentUser.Name(),
		Surname:                  currentUser.Surname(),
		Email:                    currentUser.Email(),
		Role:                     currentUser.Role(),
		CalendarConnectionStatus: calendarConnectionStatus,
	}
	if profilePhoto := currentUser.ProfilePhoto(); profilePhoto != nil {
		response.ProfilePhoto = &currentUserProfilePhotoResponse{
			OriginalName: profilePhoto.OriginalName,
			URL:          profilePhoto.URL,
		}
	}

	return response
}

func withIdentityVerificationStatus(response any, status identityverification.VerificationStatus) (any, error) {
	return withIdentityVerificationDetails(response, identityverification.VerificationStatusDetails{Status: status})
}

func withIdentityVerificationDetails(response any, details identityverification.VerificationStatusDetails) (any, error) {
	providerResponse, ok := response.(providerCurrentUserResponse)
	if !ok {
		return nil, fmt.Errorf("mapping identity verification status for unsupported user response %T", response)
	}
	providerResponse.IdentityVerificationStatus = string(details.Status)
	providerResponse.IdentityVerifiedOn = copyTime(details.VerifiedOn)
	return providerResponse, nil
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
