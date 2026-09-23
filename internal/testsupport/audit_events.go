package testsupport

import (
	"context"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
)

// AuditEvents is a test fixture and assertion helper. It deliberately uses the
// same ports as producing use cases rather than accessing the audit table or
// an administrative HTTP endpoint directly.
type AuditEvents struct {
	Writer audit.Writer
	Reader audit.Reader
}

// Save constructs a valid audit event and persists it through the writer.
// Callers provide their own context so the helper also works within a test
// transaction's lifetime.
func (f AuditEvents) Save(ctx context.Context, params audit.EventParams) (*audit.Event, error) {
	event, err := audit.NewEvent(params)
	if err != nil {
		return nil, fmt.Errorf("build audit fixture: %w", err)
	}
	if err := f.Writer.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("save audit fixture: %w", err)
	}
	return event, nil
}

// FindByID reads a persisted event via the audit reader, not through HTTP.
func (f AuditEvents) FindByID(ctx context.Context, id uuid.UUID) (*audit.Event, error) {
	event, err := f.Reader.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find audit fixture: %w", err)
	}
	return event, nil
}
