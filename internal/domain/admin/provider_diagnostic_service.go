package admin

import (
	"context"
	"errors"
	"fmt"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
	"strconv"
)

var ErrProviderDiagnosticNotFound = errors.New("provider not found")
var ErrInvalidDiagnosticProviderID = errors.New("invalid provider ID")

type ProviderDiagnosticReader interface {
	FindByProviderID(ctx context.Context, providerID int) (*readmodel.ProviderDiagnostic, error)
}
type DiagnosticService struct {
	reader    ProviderDiagnosticReader
	photos    ProfilePhotoURLResolver
	operators audit.OperatorIDFinder
	writer    audit.Writer
	clock     clock.Clock
}

func NewDiagnosticService(reader ProviderDiagnosticReader, photos ProfilePhotoURLResolver, operators audit.OperatorIDFinder, writer audit.Writer, clock clock.Clock) *DiagnosticService {
	return &DiagnosticService{reader, photos, operators, writer, clock}
}
func (s *DiagnosticService) Query(ctx context.Context, rawID, subject, correlation string) (*readmodel.ProviderDiagnostic, error) {
	id64, err := strconv.ParseInt(rawID, 10, 32)
	if err != nil || id64 <= 0 {
		return nil, ErrInvalidDiagnosticProviderID
	}
	d, err := s.reader.FindByProviderID(ctx, int(id64))
	if err != nil {
		return nil, fmt.Errorf("reading provider diagnostic: %w", err)
	}
	if d == nil {
		return nil, ErrProviderDiagnosticNotFound
	}
	if d.Provider.ProfilePhotoFileID != "" {
		urls, err := s.photos.ResolvePublicURLs(ctx, []string{d.Provider.ProfilePhotoFileID})
		if err != nil {
			return nil, fmt.Errorf("resolving diagnostic profile photo: %w", err)
		}
		d.Provider.ProfilePhotoURL = urls[d.Provider.ProfilePhotoFileID]
	}
	now := s.clock.Now()
	d.DiagnosticChecks = d.Checks(now)
	operatorID, err := s.operators.FindOperatorIDByAuthID(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("resolving diagnostic operator: %w", err)
	}
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operatorID, Action: audit.ActionAccess, ResourceType: "provider", ResourceID: strconv.Itoa(d.Provider.ID), OccurredOn: now, Result: audit.ResultPrepared, CorrelationID: correlation})
	if err != nil {
		return nil, fmt.Errorf("preparing provider diagnostic access: %w", err)
	}
	if err := s.writer.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("saving provider diagnostic access: %w", err)
	}
	return d, nil
}
