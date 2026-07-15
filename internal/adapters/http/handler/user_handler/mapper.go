package user_handler

import (
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

func currentUserResponseFromDomain(currentUser user.User) (any, error) {
	baseResponse := baseCurrentUserResponseFromDomain(currentUser.Base())

	switch typedUser := currentUser.(type) {
	case *consumer.Consumer:
		return consumerCurrentUserResponse{currentUserResponse: baseResponse}, nil
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

func baseCurrentUserResponseFromDomain(currentUser *user.BaseUser) currentUserResponse {
	response := currentUserResponse{
		ID:      currentUser.ID,
		Name:    currentUser.Name,
		Surname: currentUser.Surname,
		Email:   currentUser.Email,
		Role:    currentUser.Role,
	}
	if currentUser.ProfilePhoto != nil {
		response.ProfilePhoto = &currentUserProfilePhotoResponse{
			OriginalName: currentUser.ProfilePhoto.OriginalName,
			URL:          currentUser.ProfilePhoto.URL,
		}
	}

	return response
}
