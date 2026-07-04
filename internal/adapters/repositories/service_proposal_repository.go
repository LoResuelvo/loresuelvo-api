package repositories

import (
	"database/sql"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
)

type ServiceProposalRepository struct {
	db *sql.DB
}

func NewServiceProposalRepository(db *sql.DB) *ServiceProposalRepository {
	return &ServiceProposalRepository{db: db}
}

func (r *ServiceProposalRepository) Save(serviceProposal *serviceproposal.ServiceProposal) (*serviceproposal.ServiceProposal, error) {
	return serviceProposal, nil
}
