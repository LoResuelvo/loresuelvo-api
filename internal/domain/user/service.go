package user

import (
	"context"
	"fmt"
)

type Service struct {
	repository              Repository
	profilePhotoURLResolver ProfilePhotoURLResolver
}

func NewService(repository Repository, profilePhotoURLResolver ProfilePhotoURLResolver) *Service {
	return &Service{repository: repository, profilePhotoURLResolver: profilePhotoURLResolver}
}

func (s *Service) GetCurrentUser(ctx context.Context, authID string) (User, error) {
	currentUser, err := s.repository.FindByAuthID(authID)
	if err != nil {
		return nil, ErrNotFound
	}

	profilePhoto := currentUser.ProfilePhoto()
	if profilePhoto == nil {
		return currentUser, nil
	}

	profilePhotoURL, err := s.profilePhotoURLResolver.ResolvePublicURL(ctx, profilePhoto.FileID)
	if err != nil {
		return nil, fmt.Errorf("resolving current user profile photo URL: %w", err)
	}
	currentUser.SetProfilePhotoURL(profilePhotoURL)

	return currentUser, nil
}
