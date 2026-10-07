package claim

import (
	"context"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type Repository interface {
	Save(context.Context, *Claim) error
	FindBySubmissionKey(context.Context, int, string) (*Claim, error)
	FindOwnedByID(context.Context, int, int) (*Claim, error)
	FindOwnedPage(context.Context, int, ListCriteria) (*Page, error)
}
type UserFinder interface {
	FindClaimantByAuthID(context.Context, string) (*Claimant, error)
}
type OperationReferenceResolver interface {
	ResolveClaimOperationReference(context.Context, Claimant, Reference) (*operationmodel.ID, error)
}
type EvidenceImages interface {
	ValidateClaimEvidenceImages(context.Context, string, []string) error
	ResolveClaimEvidenceImage(context.Context, string, string) (string, error)
}
type ListCriteria struct {
	Status      *Status
	Page, Limit int
}

func (c ListCriteria) Normalize() (ListCriteria, error) {
	if c.Page == 0 {
		c.Page = 1
	}
	if c.Limit == 0 {
		c.Limit = 20
	}
	if c.Page < 1 || c.Page > 2147483647 || c.Limit < 1 || c.Limit > 100 || (c.Status != nil && !c.Status.Valid()) {
		return ListCriteria{}, ErrInvalidCriteria
	}
	return c, nil
}

type Page struct {
	Claims             []readmodel.ClaimSummary
	Page, Limit, Total int
}
type SubmissionResult struct {
	Claim   *Claim
	Created bool
}
