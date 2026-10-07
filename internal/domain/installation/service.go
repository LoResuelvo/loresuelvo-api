package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type Repository interface {
	FindByID(context.Context, string) (*Installation, error)
	Save(context.Context, *Installation) error
}
type UserFinder interface {
	FindByAuthID(string) (user.User, error)
}
type Service struct {
	repository Repository
	users      UserFinder
}

func NewService(repository Repository, users UserFinder) *Service {
	return &Service{repository: repository, users: users}
}
func (s *Service) Register(ctx context.Context, authID string, r Registration) (*Installation, bool, error) {
	if err := r.Validate(); err != nil {
		return nil, false, err
	}
	actor, err := s.users.FindByAuthID(authID)
	if err != nil {
		return nil, false, ErrForbidden
	}
	found, err := s.repository.FindByID(ctx, r.ID)
	created := errors.Is(err, ErrNotFound)
	if err != nil && !created {
		return nil, false, fmt.Errorf("finding installation: %w", err)
	}
	if created {
		found, err = NewRegistered(actor.ID(), actor.Role(), r)
	} else {
		err = found.Register(actor.ID(), actor.Role(), r)
	}
	if err != nil {
		return nil, false, err
	}
	if err := s.repository.Save(ctx, found); err != nil {
		return nil, false, fmt.Errorf("saving installation registration: %w", err)
	}
	return found, created, nil
}
func (s *Service) Unregister(ctx context.Context, authID, id, secret, bindingID string) error {
	if !validUUID(id) || !validUUID(secret) || !validUUID(bindingID) {
		return ErrInvalidInstallation
	}
	actor, err := s.users.FindByAuthID(authID)
	if err != nil {
		return ErrForbidden
	}
	found, err := s.repository.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("finding installation: %w", err)
	}
	if err := found.Unregister(actor.ID(), secret, bindingID); err != nil {
		return err
	}
	if err := s.repository.Save(ctx, found); err != nil {
		return fmt.Errorf("saving installation removal: %w", err)
	}
	return nil
}
