package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
)

type ConversationSummaryReader struct {
	db *sql.DB
}

func NewConversationSummaryReader(db *sql.DB) *ConversationSummaryReader {
	return &ConversationSummaryReader{db: db}
}

func (reader *ConversationSummaryReader) FindByConsumerID(ctx context.Context, consumerID int) ([]readmodel.ConversationSummary, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT
			c.id,
			c.status,
			p.id,
			u.name,
			u.surname,
			COALESCE(cat.name, ''),
			lm.id,
			lm.sender_role,
			lm.content,
			lm.created_on,
			c.updated_on
		FROM conversations c
		INNER JOIN providers p ON p.id = c.provider_id
		INNER JOIN users u ON u.id = p.user_id
		LEFT JOIN categories cat ON cat.id = p.category_id
		LEFT JOIN LATERAL (
			SELECT m.id, m.sender_role, m.content, m.created_on
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.created_on DESC, m.id DESC
			LIMIT 1
		) lm ON true
		WHERE c.consumer_id = $1
		ORDER BY c.updated_on DESC, c.id DESC`,
		consumerID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding consumer conversation summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanConversationSummaries(rows, conversation.SenderProvider)
}

func (reader *ConversationSummaryReader) FindByProviderID(ctx context.Context, providerID int) ([]readmodel.ConversationSummary, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT
			c.id,
			c.status,
			consumer.id,
			u.name,
			u.surname,
			'',
			lm.id,
			lm.sender_role,
			lm.content,
			lm.created_on,
			c.updated_on
		FROM conversations c
		INNER JOIN consumers consumer ON consumer.id = c.consumer_id
		INNER JOIN users u ON u.id = consumer.user_id
		LEFT JOIN LATERAL (
			SELECT m.id, m.sender_role, m.content, m.created_on
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.created_on DESC, m.id DESC
			LIMIT 1
		) lm ON true
		WHERE c.provider_id = $1
		ORDER BY c.updated_on DESC, c.id DESC`,
		providerID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding provider conversation summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanConversationSummaries(rows, conversation.SenderConsumer)
}

func scanConversationSummaries(rows *sql.Rows, counterpartRole string) ([]readmodel.ConversationSummary, error) {
	summaries := []readmodel.ConversationSummary{}
	for rows.Next() {
		var summary readmodel.ConversationSummary
		var lastMessageID sql.NullInt64
		var lastMessageSenderRole sql.NullString
		var lastMessageContent sql.NullString
		var lastMessageCreatedOn sql.NullTime

		if err := rows.Scan(
			&summary.ID,
			&summary.Status,
			&summary.Counterpart.ID,
			&summary.Counterpart.Name,
			&summary.Counterpart.Surname,
			&summary.Counterpart.CategoryName,
			&lastMessageID,
			&lastMessageSenderRole,
			&lastMessageContent,
			&lastMessageCreatedOn,
			&summary.UpdatedOn,
		); err != nil {
			return nil, fmt.Errorf("scanning conversation summary: %w", err)
		}

		summary.Counterpart.Role = counterpartRole
		if lastMessageID.Valid {
			summary.LastMessage = &readmodel.MessageSummary{
				ID:         int(lastMessageID.Int64),
				SenderRole: lastMessageSenderRole.String,
				Content:    lastMessageContent.String,
				CreatedOn:  lastMessageCreatedOn.Time,
			}
		}

		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating conversation summaries: %w", err)
	}

	return summaries, nil
}
