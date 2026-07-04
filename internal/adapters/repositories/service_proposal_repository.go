package repositories

import (
	"context"
	"database/sql"
	"fmt"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
)

type ServiceProposalRepository struct {
	db *sql.DB
}

func NewServiceProposalRepository(db *sql.DB) *ServiceProposalRepository {
	return &ServiceProposalRepository{db: db}
}

func (r *ServiceProposalRepository) Save(serviceProposal *serviceproposal.ServiceProposal) (*serviceproposal.ServiceProposal, error) {
	if serviceProposal == nil {
		return nil, fmt.Errorf("saving service proposal: service proposal is required")
	}
	if serviceProposal.Provider == nil {
		return nil, fmt.Errorf("saving service proposal: provider is required")
	}
	if serviceProposal.Consumer == nil {
		return nil, fmt.Errorf("saving service proposal: consumer is required")
	}
	if serviceProposal.Conversation == nil {
		return nil, fmt.Errorf("saving service proposal: conversation is required")
	}

	saved := *serviceProposal
	err := r.db.QueryRowContext(
		context.Background(),
		`INSERT INTO service_proposals (
			consumer_id,
			provider_id,
			conversation_id,
			amount_cents,
			scheduled_on,
			description,
			status,
			created_on,
			updated_on
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id`,
		serviceProposal.Consumer.ID,
		serviceProposal.Provider.ID,
		serviceProposal.Conversation.Base().ID,
		serviceProposal.Amount,
		serviceProposal.ScheduledOn,
		serviceProposal.Description,
		serviceProposal.Status,
	).Scan(&saved.ID)
	if err != nil {
		return nil, fmt.Errorf("saving service proposal: %w", err)
	}

	return &saved, nil
}
