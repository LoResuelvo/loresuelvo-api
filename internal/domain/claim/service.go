package claim

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
)

type Service struct {
	repo       Repository
	users      UserFinder
	operations OperationReferenceResolver
	images     EvidenceImages
	clock      clock.Clock
}

func NewService(repo Repository, users UserFinder, operations OperationReferenceResolver, images EvidenceImages, clock clock.Clock) *Service {
	return &Service{repo: repo, users: users, operations: operations, images: images, clock: clock}
}
func (s *Service) claimant(ctx context.Context, authID string) (*Claimant, error) {
	found, err := s.users.FindClaimantByAuthID(ctx, authID)
	if err != nil {
		return nil, fmt.Errorf("finding claim account: %w", err)
	}
	if found == nil || found.ID <= 0 || (found.Party != PartyConsumer && found.Party != PartyProvider) {
		return nil, ErrForbidden
	}
	return found, nil
}
func (s *Service) replay(ctx context.Context, claimantID int, key string, input Submission) (*SubmissionResult, error) {
	found, err := s.repo.FindBySubmissionKey(ctx, claimantID, key)
	if err != nil {
		return nil, fmt.Errorf("finding prior claim submission: %w", err)
	}
	if found == nil {
		return nil, nil
	}
	if !found.MatchesSubmission(input) {
		return nil, ErrSubmissionKeyConflict
	}
	return &SubmissionResult{Claim: found, Created: false}, nil
}
func (s *Service) Submit(ctx context.Context, authID, idempotencyKey string, input Submission) (*SubmissionResult, error) {
	claimant, err := s.claimant(ctx, authID)
	if err != nil {
		return nil, err
	}
	key, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	normalized, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	prior, err := s.replay(ctx, claimant.ID, key, normalized)
	if err != nil || prior != nil {
		return prior, err
	}
	operationID, err := s.operations.ResolveClaimOperationReference(ctx, *claimant, normalized.Reference)
	if err != nil {
		return nil, fmt.Errorf("resolving claim operation: %w", err)
	}
	if operationID == nil {
		return nil, ErrNotFound
	}
	if len(normalized.ImageFileIDs) > 0 {
		if err := s.images.ValidateClaimEvidenceImages(ctx, authID, normalized.ImageFileIDs); err != nil {
			return nil, fmt.Errorf("validating claim evidence: %w", err)
		}
	}
	found, err := New(*claimant, *operationID, key, normalized, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, found); err != nil {
		if errors.Is(err, ErrPersistenceConflict) || errors.Is(err, ErrOpenClaimConflict) || errors.Is(err, ErrInvalidEvidence) {
			prior, replayErr := s.replay(ctx, claimant.ID, key, normalized)
			if replayErr != nil || prior != nil {
				return prior, replayErr
			}
		}
		return nil, fmt.Errorf("saving claim: %w", err)
	}
	return &SubmissionResult{Claim: found, Created: true}, nil
}
func (s *Service) List(ctx context.Context, authID string, criteria ListCriteria) (*Page, error) {
	claimant, err := s.claimant(ctx, authID)
	if err != nil {
		return nil, err
	}
	criteria, err = criteria.Normalize()
	if err != nil {
		return nil, err
	}
	page, err := s.repo.FindOwnedPage(ctx, claimant.ID, criteria)
	if err != nil {
		return nil, fmt.Errorf("listing owned claims: %w", err)
	}
	return page, nil
}
func (s *Service) Get(ctx context.Context, authID string, id int) (*Claim, error) {
	claimant, err := s.claimant(ctx, authID)
	if err != nil {
		return nil, err
	}
	if id <= 0 || id > 2147483647 {
		return nil, ErrNotFound
	}
	found, err := s.repo.FindOwnedByID(ctx, claimant.ID, id)
	if err != nil {
		return nil, fmt.Errorf("finding owned claim: %w", err)
	}
	if found == nil {
		return nil, ErrNotFound
	}
	return found, nil
}
func (s *Service) ResolveImage(ctx context.Context, authID string, claimID int, fileID string) (string, error) {
	found, err := s.Get(ctx, authID, claimID)
	if err != nil {
		return "", err
	}
	parsed, err := uuid.Parse(fileID)
	if err != nil || len(fileID) != 36 || !found.HasImage(parsed.String()) {
		return "", ErrNotFound
	}
	url, err := s.images.ResolveClaimEvidenceImage(ctx, authID, parsed.String())
	if err != nil {
		return "", fmt.Errorf("resolving claim evidence: %w", err)
	}
	return url, nil
}
