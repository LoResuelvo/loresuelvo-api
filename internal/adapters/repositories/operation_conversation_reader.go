package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type OperationConversationReader struct{ db *sql.DB }

func NewOperationConversationReader(db *sql.DB) *OperationConversationReader {
	return &OperationConversationReader{db: db}
}

const operationConversationAssociationSQL = operationSelectedSQL + `
SELECT selected.conversation_id, selected.consumer_id, selected.provider_id, selected.job_request_id, selected.proposal_id
FROM selected
JOIN conversations c ON c.id = selected.conversation_id AND c.type = 'work'
JOIN work_conversations wc ON wc.conversation_id = c.id
 AND wc.consumer_id = selected.consumer_id AND wc.provider_id = selected.provider_id`

func (reader *OperationConversationReader) FindConversationAssociation(ctx context.Context, id readmodel.ID) (*readmodel.ConversationAssociation, error) {
	var found readmodel.ConversationAssociation
	var requestID, proposalID sql.NullInt64
	err := reader.db.QueryRowContext(ctx, operationConversationAssociationSQL, string(id.Kind), id.ResourceID).Scan(&found.ConversationID, &found.ConsumerID, &found.ProviderID, &requestID, &proposalID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying operation conversation association: %w", err)
	}
	if requestID.Valid {
		value := int(requestID.Int64)
		found.JobRequestID = &value
	}
	if proposalID.Valid {
		value := int(proposalID.Int64)
		found.ServiceProposalID = &value
	}
	return &found, nil
}
