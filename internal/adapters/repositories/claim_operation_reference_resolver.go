package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type ClaimUserFinder struct{ db *sql.DB }

func NewClaimUserFinder(db *sql.DB) *ClaimUserFinder { return &ClaimUserFinder{db: db} }
func (f *ClaimUserFinder) FindClaimantByAuthID(ctx context.Context, authID string) (*claim.Claimant, error) {
	var found claim.Claimant
	err := f.db.QueryRowContext(ctx, `SELECT u.id,u.role FROM users u WHERE u.auth_id=$1 AND
 ((u.role='consumer' AND EXISTS(SELECT 1 FROM consumers c WHERE c.user_id=u.id)) OR
 (u.role='provider' AND EXISTS(SELECT 1 FROM providers p WHERE p.user_id=u.id)))`, authID).Scan(&found.ID, &found.Party)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying claimant account: %w", err)
	}
	return &found, nil
}

type ClaimOperationReferenceResolver struct{ db *sql.DB }

func NewClaimOperationReferenceResolver(db *sql.DB) *ClaimOperationReferenceResolver {
	return &ClaimOperationReferenceResolver{db: db}
}

const claimReferenceProposalSQL = `SELECT sp.id FROM service_proposals sp
 WHERE sp.id=$1::integer AND (($2::text='consumer' AND sp.consumer_id=$3) OR ($2::text='provider' AND sp.provider_id=$3))`
const claimReferenceWorkOrderSQL = `SELECT sp.id FROM work_orders wo JOIN service_proposals sp ON sp.id=wo.service_proposal_id
 WHERE wo.id=$1::integer AND (($2::text='consumer' AND sp.consumer_id=$3) OR ($2::text='provider' AND sp.provider_id=$3))`
const claimReferencePaymentIntentSQL = `SELECT sp.id FROM payment_intents pi JOIN service_proposals sp ON sp.id=pi.service_proposal_id
 WHERE pi.id=$1::uuid AND $2::text='consumer' AND sp.consumer_id=$3`

const claimRequestForProposalSQL = `SELECT jr.id FROM job_requests jr JOIN service_proposals sp ON sp.conversation_id=jr.conversation_id WHERE sp.id=$1`

func (r *ClaimOperationReferenceResolver) ResolveClaimOperationReference(ctx context.Context, claimant claim.Claimant, reference claim.Reference) (*operationmodel.ID, error) {
	var id operationmodel.ID
	if reference.Kind() == claim.ReferenceKindJobRequest {
		err := r.db.QueryRowContext(ctx, operationSelectedSQL+` SELECT kind,resource_id FROM selected WHERE ($3::text='consumer' AND consumer_id=$4) OR ($3::text='provider' AND provider_id=$4)`, "jr", reference.ID(), claimant.Party, claimant.ID).Scan(&id.Kind, &id.ResourceID)
		return claimResolvedOperation(id, err)
	}
	query := claimReferenceProposalSQL
	switch reference.Kind() {
	case claim.ReferenceKindServiceProposal:
	case claim.ReferenceKindWorkOrder:
		query = claimReferenceWorkOrderSQL
	case claim.ReferenceKindPaymentIntent:
		query = claimReferencePaymentIntentSQL
	default:
		return nil, claim.ErrInvalidSubmission
	}
	var proposalID int
	err := r.db.QueryRowContext(ctx, query, reference.ID(), claimant.Party, claimant.ID).Scan(&proposalID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying authorized claim reference: %w", err)
	}
	var requestID int
	err = r.db.QueryRowContext(ctx, claimRequestForProposalSQL, proposalID).Scan(&requestID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("querying claim proposal origin: %w", err)
	}
	if err == nil {
		err = r.db.QueryRowContext(ctx, operationSelectedSQL+` SELECT kind,resource_id FROM selected WHERE proposal_id=$3`, "jr", requestID, proposalID).Scan(&id.Kind, &id.ResourceID)
		if err == nil {
			return &id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("querying canonical claim origin: %w", err)
		}
	}
	err = r.db.QueryRowContext(ctx, operationSelectedSQL+` SELECT kind,resource_id FROM selected`, "sp", proposalID).Scan(&id.Kind, &id.ResourceID)
	return claimResolvedOperation(id, err)
}
func claimResolvedOperation(id operationmodel.ID, err error) (*operationmodel.ID, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying canonical claim operation: %w", err)
	}
	return &id, nil
}
