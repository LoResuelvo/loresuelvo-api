package claim

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
)

type AdminCriteria struct {
	ListCriteria
	Query string
}

func (c AdminCriteria) Normalize() (AdminCriteria, error) {
	base, err := c.ListCriteria.Normalize()
	if err != nil {
		return AdminCriteria{}, err
	}
	c.ListCriteria = base
	c.Query = strings.TrimSpace(c.Query)
	if !utf8.ValidString(c.Query) || strings.ContainsRune(c.Query, 0) {
		return AdminCriteria{}, ErrInvalidCriteria
	}
	return c, nil
}

type AdminPage struct {
	Claims             []readmodel.AdminClaimSummary
	Page, Limit, Total int
}
type AdminDetail struct {
	GetResult
	ClaimantEmail string
	CategoryID    *int
	CategoryName  *string
}
type AdminReader interface {
	FindAdministrativePage(context.Context, AdminCriteria) (*AdminPage, error)
	FindAdministrativeByID(context.Context, int) (*AdminDetail, error)
}
type AdministrativeEvidenceImages interface {
	ResolveAdministrativeClaimEvidenceImage(context.Context, string) (string, error)
}
type AdministrationRecord struct {
	OperatorID        int
	Key, Fingerprint  string
	ClaimID, ActionID int
}
type AdministrationResult struct {
	Claim  *Claim
	Action Action
}
type AdministrationStore interface {
	FindClaim(context.Context, int) (*Claim, error)
	FindRecord(context.Context, int, string) (*AdministrationRecord, error)
	SaveClaim(context.Context, *Claim) error
	SaveRecord(context.Context, *AdministrationRecord) error
	SaveAuditEvent(context.Context, *audit.Event) error
}
type AdministrationUnitOfWork interface {
	Execute(context.Context, func(AdministrationStore) error) error
}
