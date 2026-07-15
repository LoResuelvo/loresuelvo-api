package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
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

func (r *ServiceProposalRepository) FindByID(ctx context.Context, id int) (*serviceproposal.ServiceProposal, error) {
	var (
		proposal   serviceproposal.ServiceProposal
		consumerID int
		providerID int
	)
	err := r.db.QueryRowContext(
		ctx,
		`SELECT
			sp.id,
			sp.amount_cents,
			sp.scheduled_on,
			sp.description,
			sp.status,
			sp.created_on,
			sp.consumer_id,
			sp.provider_id
		FROM service_proposals sp
		WHERE sp.id = $1`,
		id,
	).Scan(
		&proposal.ID,
		&proposal.Amount,
		&proposal.ScheduledOn,
		&proposal.Description,
		&proposal.Status,
		&proposal.CreatedOn,
		&consumerID,
		&providerID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, serviceproposal.ErrDoesNotExist
		}
		return nil, fmt.Errorf("finding service proposal by id: %w", err)
	}

	proposal.Consumer = &consumer.Consumer{BaseUser: &user.BaseUser{ID: consumerID, Role: consumer.Role}}
	proposal.Provider = &provider.Provider{BaseUser: &user.BaseUser{ID: providerID, Role: provider.Role}}
	return &proposal, nil
}

func (r *ServiceProposalRepository) updateAcceptedWithTx(
	ctx context.Context,
	tx *sql.Tx,
	proposal *serviceproposal.ServiceProposal,
) error {
	if tx == nil {
		return fmt.Errorf("updating accepted service proposal: transaction is required")
	}
	if proposal == nil {
		return fmt.Errorf("updating accepted service proposal: service proposal is required")
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE service_proposals
		SET status = $1, updated_on = NOW()
		WHERE id = $2 AND status = $3`,
		proposal.Status,
		proposal.ID,
		serviceproposal.StatusPending,
	)
	if err != nil {
		return fmt.Errorf("updating accepted service proposal: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking accepted service proposal update: %w", err)
	}
	if affected != 1 {
		return serviceproposal.ErrOnlyPendingCanBeAccepted
	}
	return nil
}

func (r *ServiceProposalRepository) FindByUserID(ctx context.Context, userID int) ([]*serviceproposal.ServiceProposal, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			sp.id,
			sp.amount_cents,
			sp.scheduled_on,
			sp.description,
			sp.status,
			sp.created_on,
			sp.conversation_id,
			c.type,
			c.status,
			consumer_user.id,
			consumer_user.auth_id,
			consumer_user.email,
			consumer_user.name,
			consumer_user.surname,
			provider_user.id,
			provider_user.auth_id,
			provider_user.email,
			provider_user.name,
			provider_user.surname,
			cat.id,
			cat.name,
			cat.normalized_name,
			provider_user.profile_photo_file_id
		FROM service_proposals sp
		INNER JOIN conversations c ON c.id = sp.conversation_id
		INNER JOIN users consumer_user ON consumer_user.id = sp.consumer_id
		INNER JOIN providers p ON p.user_id = sp.provider_id
		INNER JOIN users provider_user ON provider_user.id = p.user_id
		INNER JOIN categories cat ON cat.id = p.category_id
		WHERE sp.consumer_id = $1 OR sp.provider_id = $1
		ORDER BY sp.created_on DESC, sp.id DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding service proposals by user id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	proposals := make([]*serviceproposal.ServiceProposal, 0)
	for rows.Next() {
		var (
			proposal           serviceproposal.ServiceProposal
			baseConversation   conversation.BaseConversation
			consumerUser       user.BaseUser
			providerUser       user.BaseUser
			providerCategory   category.Category
			profilePhotoFileID string
		)

		err := rows.Scan(
			&proposal.ID,
			&proposal.Amount,
			&proposal.ScheduledOn,
			&proposal.Description,
			&proposal.Status,
			&proposal.CreatedOn,
			&baseConversation.ID,
			&baseConversation.Type,
			&baseConversation.Status,
			&consumerUser.ID,
			&consumerUser.AuthID,
			&consumerUser.Email,
			&consumerUser.Name,
			&consumerUser.Surname,
			&providerUser.ID,
			&providerUser.AuthID,
			&providerUser.Email,
			&providerUser.Name,
			&providerUser.Surname,
			&providerCategory.ID,
			&providerCategory.Name,
			&providerCategory.NormalizedName,
			&profilePhotoFileID,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning service proposal by user id: %w", err)
		}

		consumerUser.Role = consumer.Role
		providerUser.Role = provider.Role
		providerUser.ProfilePhoto = imageFromPersistence(profilePhotoFileID, "")
		proposal.Consumer = &consumer.Consumer{BaseUser: &consumerUser}
		proposal.Provider = &provider.Provider{
			BaseUser: &providerUser,
			Category: &providerCategory,
		}
		proposal.Conversation = &conversation.WorkConversation{
			BaseConversation: &baseConversation,
			ConsumerID:       consumerUser.ID,
			ProviderID:       providerUser.ID,
		}
		proposals = append(proposals, &proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating service proposals by user id: %w", err)
	}

	return proposals, nil
}
