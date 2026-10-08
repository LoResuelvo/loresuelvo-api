package claim

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
)

type AdminService struct {
	reader    AdminReader
	operators audit.OperatorIDFinder
	unit      AdministrationUnitOfWork
	writer    audit.Writer
	images    AdministrativeEvidenceImages
	clock     clock.Clock
}

func NewAdminService(reader AdminReader, operators audit.OperatorIDFinder, unit AdministrationUnitOfWork, writer audit.Writer, images AdministrativeEvidenceImages, clock clock.Clock) *AdminService {
	return &AdminService{reader: reader, operators: operators, unit: unit, writer: writer, images: images, clock: clock}
}

func (s *AdminService) List(ctx context.Context, criteria AdminCriteria) (*AdminPage, error) {
	criteria, err := criteria.Normalize()
	if err != nil {
		return nil, err
	}
	page, err := s.reader.FindAdministrativePage(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("reading administrative claims: %w", err)
	}
	now := s.clock.Now()
	for i := range page.Claims {
		age := now.Sub(page.Claims[i].CreatedOn)
		if age > 0 {
			page.Claims[i].AgeSeconds = int64(age.Seconds())
		}
	}
	return page, nil
}

func (s *AdminService) Get(ctx context.Context, auth string, id int, correlation string) (*AdminDetail, error) {
	operator, err := s.operators.FindOperatorIDByAuthID(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("finding claim operator: %w", err)
	}
	if operator <= 0 {
		return nil, ErrForbidden
	}
	found, err := s.reader.FindAdministrativeByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("reading administrative claim: %w", err)
	}
	if found == nil {
		return nil, ErrNotFound
	}
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operator, Action: audit.ActionAccess, ResourceType: "claim", ResourceID: strconv.Itoa(id), OccurredOn: s.clock.Now(), Result: audit.ResultPrepared, CorrelationID: correlation})
	if err != nil {
		return nil, err
	}
	if err := s.writer.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("saving claim access audit: %w", err)
	}
	found.Images = make([]readmodel.EvidenceImage, 0, len(found.Claim.ImageFileIDs))
	for _, fileID := range found.Claim.ImageFileIDs {
		url, err := s.images.ResolveAdministrativeClaimEvidenceImage(ctx, fileID)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrEvidenceAccessUnavailable, err)
		}
		if url == "" {
			return nil, ErrEvidenceAccessUnavailable
		}
		found.Images = append(found.Images, readmodel.EvidenceImage{FileID: fileID, URL: url})
	}
	return found, nil
}

func (s *AdminService) StartReview(ctx context.Context, auth string, id int, key, correlation string) (*AdministrationResult, error) {
	return s.execute(ctx, auth, id, key, correlation, "review_started", ResolutionInput{})
}

func (s *AdminService) Resolve(ctx context.Context, auth string, id int, key, correlation string, input ResolutionInput) (*AdministrationResult, error) {
	normalized, err := input.Normalize()
	if err != nil {
		return nil, err
	}
	return s.execute(ctx, auth, id, key, correlation, "resolved", normalized)
}

func (s *AdminService) execute(ctx context.Context, auth string, id int, key, correlation, actionType string, input ResolutionInput) (*AdministrationResult, error) {
	key, err := NormalizeIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	operator, err := s.operators.FindOperatorIDByAuthID(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("finding claim operator: %w", err)
	}
	if operator <= 0 {
		return nil, ErrForbidden
	}
	fingerprint := administrationFingerprint(id, actionType, input)
	var result *AdministrationResult
	operation := func(store AdministrationStore) error {
		found, err := store.FindClaim(ctx, id)
		if err != nil {
			return err
		}
		if found == nil {
			return ErrNotFound
		}
		record, err := store.FindRecord(ctx, operator, key)
		if err != nil {
			return err
		}
		if record != nil {
			if record.ClaimID != id || record.Fingerprint != fingerprint {
				return ErrAdministrationKeyConflict
			}
			for _, action := range found.Actions {
				if action.ID == record.ActionID {
					result = &AdministrationResult{Claim: found, Action: action}
					return nil
				}
			}
			return fmt.Errorf("reading original claim action: %w", ErrPersistenceConflict)
		}
		before := found.Status
		var action *Action
		if actionType == "review_started" {
			action, err = found.StartReview(operator, s.clock.Now())
		} else {
			action, err = found.Resolve(operator, input, s.clock.Now())
		}
		if err != nil {
			return err
		}
		if err := store.SaveClaim(ctx, found); err != nil {
			return err
		}
		change, err := audit.NewStateChange("status", string(before), string(found.Status))
		if err != nil {
			return err
		}
		event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operator, Action: audit.ActionExecute, ResourceType: "claim", ResourceID: strconv.Itoa(id), OccurredOn: action.CreatedOn, Result: audit.ResultSucceeded, CorrelationID: correlation, StateChange: change})
		if err != nil {
			return err
		}
		if err := store.SaveAuditEvent(ctx, event); err != nil {
			return err
		}
		if err := store.SaveRecord(ctx, &AdministrationRecord{OperatorID: operator, Key: key, Fingerprint: fingerprint, ClaimID: id, ActionID: action.ID}); err != nil {
			return err
		}
		result = &AdministrationResult{Claim: found, Action: *action}
		return nil
	}
	err = s.unit.Execute(ctx, operation)
	// A competing key on a different claim can commit after our lookup. The failed transaction
	// is rolled back before re-reading the winning record; no state or audit escapes that rollback.
	if errors.Is(err, ErrAdministrationRecordConflict) {
		err = s.unit.Execute(ctx, operation)
	}
	if err != nil {
		return nil, fmt.Errorf("saving claim administration: %w", err)
	}
	return result, nil
}
