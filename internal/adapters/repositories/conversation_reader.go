package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
)

type ConversationReader struct {
	db *sql.DB
}

func NewConversationReader(db *sql.DB) *ConversationReader {
	return &ConversationReader{db: db}
}

func (reader *ConversationReader) FindSummariesByParticipantIDRoleAndType(ctx context.Context, participantID int, participantRole string, conversationType string) ([]readmodel.ConversationSummary, error) {
	switch conversationType {
	case conversation.TypeWork:
		return reader.findWorkSummariesByParticipantIDAndRole(ctx, participantID, participantRole, conversationType)
	case conversation.TypeChatbot:
		return reader.findChatbotSummariesByConsumerIDAndType(ctx, participantID, conversationType)
	default:
		return nil, fmt.Errorf("finding conversation summaries: unsupported conversation type %q", conversationType)
	}
}

func (reader *ConversationReader) findWorkSummariesByParticipantIDAndRole(ctx context.Context, participantID int, participantRole string, conversationType string) ([]readmodel.ConversationSummary, error) {
	switch participantRole {
	case conversation.SenderConsumer:
		return reader.findWorkSummariesByConsumerID(ctx, participantID, conversationType)
	case conversation.SenderProvider:
		return reader.findWorkSummariesByProviderID(ctx, participantID, conversationType)
	default:
		return nil, fmt.Errorf("finding work conversation summaries: unsupported participant role %q", participantRole)
	}
}

func (reader *ConversationReader) findWorkSummariesByConsumerID(ctx context.Context, consumerID int, conversationType string) ([]readmodel.ConversationSummary, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT
			c.id,
			c.type,
			c.status,
			p.id,
			u.name,
			u.surname,
			COALESCE(cat.name, ''),
			COALESCE(p.profile_photo_file_id::text, ''),
			lm.id,
			lm.sender_role,
			lm.content,
			lm.created_on,
			c.updated_on
		FROM conversations c
		INNER JOIN work_conversations wc ON wc.conversation_id = c.id
		INNER JOIN providers p ON p.id = wc.provider_id
		INNER JOIN users u ON u.id = p.user_id
		LEFT JOIN categories cat ON cat.id = p.category_id
		LEFT JOIN LATERAL (
			SELECT m.id, m.sender_role, m.content, m.created_on
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.created_on DESC, m.id DESC
			LIMIT 1
		) lm ON true
		WHERE wc.consumer_id = $1
			AND c.type = $2
		ORDER BY c.updated_on DESC, c.id DESC`,
		consumerID,
		conversationType,
	)
	if err != nil {
		return nil, fmt.Errorf("finding consumer work conversation summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanWorkConversationSummaries(rows, conversation.SenderProvider)
}

func (reader *ConversationReader) findWorkSummariesByProviderID(ctx context.Context, providerID int, conversationType string) ([]readmodel.ConversationSummary, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT
			c.id,
			c.type,
			c.status,
			consumer.id,
			u.name,
			u.surname,
			'',
			'',
			lm.id,
			lm.sender_role,
			lm.content,
			lm.created_on,
			c.updated_on
		FROM conversations c
		INNER JOIN work_conversations wc ON wc.conversation_id = c.id
		INNER JOIN consumers consumer ON consumer.id = wc.consumer_id
		INNER JOIN users u ON u.id = consumer.user_id
		LEFT JOIN LATERAL (
			SELECT m.id, m.sender_role, m.content, m.created_on
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.created_on DESC, m.id DESC
			LIMIT 1
		) lm ON true
		WHERE wc.provider_id = $1
			AND c.type = $2
		ORDER BY c.updated_on DESC, c.id DESC`,
		providerID,
		conversationType,
	)
	if err != nil {
		return nil, fmt.Errorf("finding provider work conversation summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanWorkConversationSummaries(rows, conversation.SenderConsumer)
}

func (reader *ConversationReader) findChatbotSummariesByConsumerIDAndType(ctx context.Context, consumerID int, conversationType string) ([]readmodel.ConversationSummary, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT
			c.id,
			c.type,
			c.status,
			cc.title,
			lm.id,
			lm.sender_role,
			lm.content,
			lm.created_on,
			c.updated_on
		FROM conversations c
		INNER JOIN chatbot_conversations cc ON cc.conversation_id = c.id
		LEFT JOIN LATERAL (
			SELECT m.id, m.sender_role, m.content, m.created_on
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.created_on DESC, m.id DESC
			LIMIT 1
		) lm ON true
		WHERE cc.consumer_id = $1
			AND c.type = $2
		ORDER BY c.updated_on DESC, c.id DESC`,
		consumerID,
		conversationType,
	)
	if err != nil {
		return nil, fmt.Errorf("finding chatbot conversation summaries by consumer and type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanChatbotConversationSummaries(rows)
}

func (reader *ConversationReader) FindDetailByIDForConsumer(ctx context.Context, conversationID int) (*readmodel.ConversationDetail, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT
			c.id,
			c.status,
			p.id,
			u.name,
			u.surname,
			COALESCE(cat.name, ''),
			COALESCE(p.profile_photo_file_id::text, ''),
			m.id,
			m.sender_role,
			m.content,
			m.created_on,
			c.updated_on
		FROM conversations c
		INNER JOIN work_conversations wc ON wc.conversation_id = c.id
		INNER JOIN providers p ON p.id = wc.provider_id
		INNER JOIN users u ON u.id = p.user_id
		LEFT JOIN categories cat ON cat.id = p.category_id
		LEFT JOIN messages m ON m.conversation_id = c.id
		WHERE c.id = $1
		ORDER BY m.created_on ASC, m.id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding consumer conversation detail: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanConversationDetail(rows, conversation.SenderProvider)
}

func (reader *ConversationReader) FindDetailByIDForProvider(ctx context.Context, conversationID int) (*readmodel.ConversationDetail, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT
			c.id,
			c.status,
			consumer.id,
			u.name,
			u.surname,
			'',
			'',
			m.id,
			m.sender_role,
			m.content,
			m.created_on,
			c.updated_on
		FROM conversations c
		INNER JOIN work_conversations wc ON wc.conversation_id = c.id
		INNER JOIN consumers consumer ON consumer.id = wc.consumer_id
		INNER JOIN users u ON u.id = consumer.user_id
		LEFT JOIN messages m ON m.conversation_id = c.id
		WHERE c.id = $1
		ORDER BY m.created_on ASC, m.id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding provider conversation detail: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanConversationDetail(rows, conversation.SenderConsumer)
}

func scanChatbotConversationSummaries(rows *sql.Rows) ([]readmodel.ConversationSummary, error) {
	summaries := []readmodel.ConversationSummary{}
	for rows.Next() {
		var summary readmodel.ConversationSummary
		var title string
		var lastMessageID sql.NullInt64
		var lastMessageSenderRole sql.NullString
		var lastMessageContent sql.NullString
		var lastMessageCreatedOn sql.NullTime

		if err := rows.Scan(
			&summary.ID,
			&summary.Type,
			&summary.Status,
			&title,
			&lastMessageID,
			&lastMessageSenderRole,
			&lastMessageContent,
			&lastMessageCreatedOn,
			&summary.UpdatedOn,
		); err != nil {
			return nil, fmt.Errorf("scanning chatbot conversation summary: %w", err)
		}

		summary.Chatbot = &readmodel.ChatbotConversationSummary{Title: title}
		setSummaryLastMessage(&summary, lastMessageID, lastMessageSenderRole, lastMessageContent, lastMessageCreatedOn)
		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating chatbot conversation summaries: %w", err)
	}

	return summaries, nil
}

func scanWorkConversationSummaries(rows *sql.Rows, counterpartRole string) ([]readmodel.ConversationSummary, error) {
	summaries := []readmodel.ConversationSummary{}
	for rows.Next() {
		var summary readmodel.ConversationSummary
		var counterpart readmodel.ConversationParticipant
		var lastMessageID sql.NullInt64
		var lastMessageSenderRole sql.NullString
		var lastMessageContent sql.NullString
		var lastMessageCreatedOn sql.NullTime

		if err := rows.Scan(
			&summary.ID,
			&summary.Type,
			&summary.Status,
			&counterpart.ID,
			&counterpart.Name,
			&counterpart.Surname,
			&counterpart.CategoryName,
			&counterpart.ProfilePhotoFileID,
			&lastMessageID,
			&lastMessageSenderRole,
			&lastMessageContent,
			&lastMessageCreatedOn,
			&summary.UpdatedOn,
		); err != nil {
			return nil, fmt.Errorf("scanning work conversation summary: %w", err)
		}

		counterpart.Role = counterpartRole
		summary.Work = &readmodel.WorkConversationSummary{Counterpart: counterpart}
		setSummaryLastMessage(&summary, lastMessageID, lastMessageSenderRole, lastMessageContent, lastMessageCreatedOn)
		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating work conversation summaries: %w", err)
	}

	return summaries, nil
}

func setSummaryLastMessage(summary *readmodel.ConversationSummary, id sql.NullInt64, senderRole sql.NullString, content sql.NullString, createdOn sql.NullTime) {
	if !id.Valid {
		return
	}

	summary.LastMessage = &readmodel.MessageSummary{
		ID:         int(id.Int64),
		SenderRole: senderRole.String,
		Content:    content.String,
		CreatedOn:  createdOn.Time,
	}
}

func scanConversationDetail(rows *sql.Rows, counterpartRole string) (*readmodel.ConversationDetail, error) {
	var detail *readmodel.ConversationDetail

	for rows.Next() {
		var rowDetail readmodel.ConversationDetail
		var messageID sql.NullInt64
		var messageSenderRole sql.NullString
		var messageContent sql.NullString
		var messageCreatedOn sql.NullTime

		if err := rows.Scan(
			&rowDetail.ID,
			&rowDetail.Status,
			&rowDetail.Counterpart.ID,
			&rowDetail.Counterpart.Name,
			&rowDetail.Counterpart.Surname,
			&rowDetail.Counterpart.CategoryName,
			&rowDetail.Counterpart.ProfilePhotoFileID,
			&messageID,
			&messageSenderRole,
			&messageContent,
			&messageCreatedOn,
			&rowDetail.UpdatedOn,
		); err != nil {
			return nil, fmt.Errorf("scanning conversation detail: %w", err)
		}

		if detail == nil {
			rowDetail.Counterpart.Role = counterpartRole
			rowDetail.Messages = []readmodel.MessageDetail{}
			detail = &rowDetail
		}

		if messageID.Valid {
			detail.Messages = append(detail.Messages, readmodel.MessageDetail{
				ID:         int(messageID.Int64),
				SenderRole: messageSenderRole.String,
				Content:    messageContent.String,
				CreatedOn:  messageCreatedOn.Time,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating conversation detail: %w", err)
	}
	if detail == nil {
		return nil, conversation.ErrConversationDoesNotExist
	}

	return detail, nil
}
