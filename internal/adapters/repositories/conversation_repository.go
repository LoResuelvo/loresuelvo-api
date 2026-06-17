package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolationCode = "23505"

type ConversationRepository struct {
	db                *sql.DB
	messageRepository *MessageRepository
}

func NewConversationRepository(db *sql.DB, messageRepository *MessageRepository) *ConversationRepository {
	return &ConversationRepository{db: db, messageRepository: messageRepository}
}

func (repository *ConversationRepository) ExistsBetween(consumerID, providerID int) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM work_conversations WHERE consumer_id = $1 AND provider_id = $2
		)`,
		consumerID,
		providerID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking conversation existence: %w", err)
	}

	return exists, nil
}

func (repository *ConversationRepository) SaveConversation(ctx context.Context, conversationToSave conversation.Conversation) (conversation.Conversation, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning conversation transaction: %w", err)
	}

	if err := repository.saveConversationWithTx(ctx, tx, conversationToSave); err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing conversation transaction: %w", err)
	}

	return conversationToSave, nil
}

func (repository *ConversationRepository) SaveWithMessage(conversationToSave conversation.Conversation, message conversation.Message) (conversation.Conversation, error) {
	conversationToSave.AddMessage(message)
	return repository.SaveConversation(context.Background(), conversationToSave)
}

func (repository *ConversationRepository) saveConversationWithTx(ctx context.Context, tx *sql.Tx, conversationToSave conversation.Conversation) error {
	base := conversationToSave.Base()
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO conversations (type, status, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, updated_on`,
		base.Type,
		base.Status,
	).Scan(
		&base.ID,
		&base.UpdatedOn,
	)
	if err != nil {
		return mapConversationInsertError(err)
	}

	messages := conversationToSave.Messages()
	baseMessages := make([]conversation.Message, 0, len(messages))
	for _, message := range messages {
		savedMessage, err := repository.messageRepository.saveWithTx(ctx, tx, base.ID, message)
		if err != nil {
			return err
		}
		baseMessages = append(baseMessages, *savedMessage)
	}
	base.SetMessages(baseMessages)

	switch typedConversation := conversationToSave.(type) {
	case *conversation.WorkConversation:
		if err := repository.saveWorkConversationWithTx(ctx, tx, typedConversation); err != nil {
			return err
		}
	case *conversation.ChatBotConversation:
		if err := repository.saveChatbotConversationWithTx(ctx, tx, typedConversation); err != nil {
			return err
		}
	default:
		return fmt.Errorf("saving conversation: unsupported conversation type %q", conversationToSave.ConversationType())
	}

	return nil
}

func (repository *ConversationRepository) AddMessage(ctx context.Context, conversationID int, message conversation.Message) (*conversation.Message, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning add message transaction: %w", err)
	}

	if err := lockConversationByID(ctx, tx, conversationID); err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	savedMessage, err := repository.messageRepository.saveWithTx(ctx, tx, conversationID, message)
	if err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	if err := repository.updateTimestampWithTx(ctx, tx, conversationID, savedMessage.CreatedOn); err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing add message transaction: %w", err)
	}

	return savedMessage, nil
}

func (repository *ConversationRepository) CountMessagesBySenderRole(ctx context.Context, conversationID int, senderRole string) (int, error) {
	var count int
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM messages
		WHERE conversation_id = $1 AND sender_role = $2`,
		conversationID,
		senderRole,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting messages by sender role: %w", err)
	}

	return count, nil
}

func (repository *ConversationRepository) UpdateConversation(ctx context.Context, conversationToUpdate conversation.Conversation) (conversation.Conversation, error) {
	base := conversationToUpdate.Base()
	if base.ID <= 0 {
		return nil, conversation.ErrConversationDoesNotExist
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning update conversation transaction: %w", err)
	}

	if err := lockConversationByID(ctx, tx, base.ID); err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	if err := repository.updateConversationStatusWithTx(ctx, tx, conversationToUpdate); err != nil {
		return nil, rollbackConversationTx(tx, err)
	}

	savedMessages, lastSavedMessage, err := repository.saveNewMessagesWithTx(ctx, tx, conversationToUpdate)
	if err != nil {
		return nil, rollbackConversationTx(tx, err)
	}
	if lastSavedMessage != nil {
		base.SetMessages(savedMessages)
		if err := repository.updateTimestampWithTx(ctx, tx, base.ID, lastSavedMessage.CreatedOn); err != nil {
			return nil, rollbackConversationTx(tx, err)
		}
	}

	if chatbotConversation, ok := conversationToUpdate.(*conversation.ChatBotConversation); ok {
		if err := repository.updateChatbotConversationWithTx(ctx, tx, chatbotConversation); err != nil {
			return nil, rollbackConversationTx(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing update conversation transaction: %w", err)
	}

	return conversationToUpdate, nil
}

func (repository *ConversationRepository) updateConversationStatusWithTx(ctx context.Context, tx *sql.Tx, conversationToUpdate conversation.Conversation) error {
	base := conversationToUpdate.Base()
	result, err := tx.ExecContext(
		ctx,
		`UPDATE conversations
		SET status = $2
		WHERE id = $1`,
		base.ID,
		base.Status,
	)
	if err != nil {
		return fmt.Errorf("updating conversation status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated conversation status rows: %w", err)
	}
	if rowsAffected == 0 {
		return conversation.ErrConversationDoesNotExist
	}

	return nil
}

func (repository *ConversationRepository) saveNewMessagesWithTx(ctx context.Context, tx *sql.Tx, conversationToUpdate conversation.Conversation) ([]conversation.Message, *conversation.Message, error) {
	conversationID := conversationToUpdate.Base().ID
	savedMessages := make([]conversation.Message, 0, len(conversationToUpdate.Messages()))
	var lastSavedMessage *conversation.Message

	for _, message := range conversationToUpdate.Messages() {
		if message.ID > 0 {
			savedMessages = append(savedMessages, message)
			continue
		}

		savedMessage, err := repository.messageRepository.saveWithTx(ctx, tx, conversationID, message)
		if err != nil {
			return nil, nil, err
		}
		savedMessages = append(savedMessages, *savedMessage)
		lastSavedMessage = savedMessage
	}

	return savedMessages, lastSavedMessage, nil
}

func (repository *ConversationRepository) updateChatbotConversationWithTx(ctx context.Context, tx *sql.Tx, chatbotConversation *conversation.ChatBotConversation) error {
	processingStartedOn := sql.NullTime{}
	if startedOn := chatbotConversation.ProcessingStartedAt(); startedOn != nil {
		processingStartedOn = sql.NullTime{Time: *startedOn, Valid: true}
	}

	if processingStartedOn.Valid {
		return repository.startChatbotProcessingWithTx(ctx, tx, chatbotConversation, processingStartedOn.Time)
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE chatbot_conversations
		SET context_summary = $2,
			last_summarized_message_id = $3,
			processing_started_on = NULL
		WHERE conversation_id = $1`,
		chatbotConversation.Base().ID,
		chatbotConversation.Context.Summary,
		chatbotConversation.Context.LastSummarizedMessageID,
	)
	if err != nil {
		return fmt.Errorf("updating chatbot conversation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated chatbot conversation rows: %w", err)
	}
	if rowsAffected == 0 {
		return conversation.ErrConversationDoesNotExist
	}

	return nil
}

func (repository *ConversationRepository) startChatbotProcessingWithTx(ctx context.Context, tx *sql.Tx, chatbotConversation *conversation.ChatBotConversation, startedOn time.Time) error {
	staleBefore := startedOn.Add(-conversation.ChatbotProcessingStaleAfter)
	result, err := tx.ExecContext(
		ctx,
		`UPDATE chatbot_conversations
		SET context_summary = $2,
			last_summarized_message_id = $3,
			processing_started_on = $4
		WHERE conversation_id = $1
			AND (processing_started_on IS NULL OR processing_started_on = $4 OR processing_started_on < $5)`,
		chatbotConversation.Base().ID,
		chatbotConversation.Context.Summary,
		chatbotConversation.Context.LastSummarizedMessageID,
		startedOn,
		staleBefore,
	)
	if err != nil {
		return fmt.Errorf("starting chatbot processing: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking chatbot processing rows: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM chatbot_conversations WHERE conversation_id = $1)`, chatbotConversation.Base().ID).Scan(&exists); err != nil {
		return fmt.Errorf("checking chatbot conversation existence: %w", err)
	}
	if !exists {
		return conversation.ErrConversationDoesNotExist
	}

	return conversation.ErrChatbotConversationAlreadyProcessing
}

func (repository *ConversationRepository) DeleteBetween(consumerID, providerID int) error {
	_, err := repository.db.Exec(
		`DELETE FROM conversations c
		USING work_conversations wc
		WHERE wc.conversation_id = c.id
			AND wc.consumer_id = $1
			AND wc.provider_id = $2`,
		consumerID,
		providerID,
	)
	if err != nil {
		return fmt.Errorf("deleting conversation between consumer and provider: %w", err)
	}

	return nil
}

func (repository *ConversationRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM conversations`)
	if err != nil {
		return fmt.Errorf("deleting all conversations: %w", err)
	}

	return nil
}

func (repository *ConversationRepository) FindBetween(consumerID, providerID int) (conversation.Conversation, error) {
	foundConversation := &conversation.WorkConversation{BaseConversation: &conversation.BaseConversation{Type: conversation.TypeWork}}
	err := repository.db.QueryRow(
		`SELECT c.id, wc.consumer_id, wc.provider_id, c.status, c.updated_on
		FROM conversations c
		INNER JOIN work_conversations wc ON wc.conversation_id = c.id
		WHERE wc.consumer_id = $1 AND wc.provider_id = $2`,
		consumerID,
		providerID,
	).Scan(
		&foundConversation.BaseConversation.ID,
		&foundConversation.ConsumerID,
		&foundConversation.ProviderID,
		&foundConversation.BaseConversation.Status,
		&foundConversation.BaseConversation.UpdatedOn,
	)
	if err != nil {
		return nil, fmt.Errorf("finding conversation between consumer and provider: %w", err)
	}

	return foundConversation, nil
}

func (repository *ConversationRepository) FindByID(ctx context.Context, conversationID int) (conversation.Conversation, error) {
	var base conversation.BaseConversation
	var consumerID int
	var providerID int
	var title string
	var contextSummary string
	var lastSummarizedMessageID int
	var processingStartedOn sql.NullTime
	var conversationType string
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT
			c.id,
			COALESCE(wc.consumer_id, cc.consumer_id),
			COALESCE(wc.provider_id, 0),
			COALESCE(cc.title, ''),
			COALESCE(cc.context_summary, ''),
			COALESCE(cc.last_summarized_message_id, 0),
			cc.processing_started_on,
			c.status,
			c.updated_on,
			c.type
		FROM conversations c
		LEFT JOIN work_conversations wc ON wc.conversation_id = c.id
		LEFT JOIN chatbot_conversations cc ON cc.conversation_id = c.id
		WHERE c.id = $1`,
		conversationID,
	).Scan(
		&base.ID,
		&consumerID,
		&providerID,
		&title,
		&contextSummary,
		&lastSummarizedMessageID,
		&processingStartedOn,
		&base.Status,
		&base.UpdatedOn,
		&conversationType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, conversation.ErrConversationDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding conversation by id: %w", err)
	}

	base.Type = conversationType
	messages, err := repository.findMessagesByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	base.SetMessages(messages)

	switch conversationType {
	case conversation.TypeChatbot:
		var processingStartedAt *time.Time
		if processingStartedOn.Valid {
			processingStartedAt = &processingStartedOn.Time
		}
		return &conversation.ChatBotConversation{
			BaseConversation: &base,
			ConsumerID:       consumerID,
			Title:            title,
			Context: conversation.ChatbotConversationContext{
				Summary:                 contextSummary,
				LastSummarizedMessageID: lastSummarizedMessageID,
			},
			ProcessingStartedOn: processingStartedAt,
		}, nil
	case conversation.TypeWork:
		return &conversation.WorkConversation{
			BaseConversation: &base,
			ConsumerID:       consumerID,
			ProviderID:       providerID,
		}, nil
	default:
		return nil, fmt.Errorf("finding conversation by id: unsupported conversation type %q", conversationType)
	}
}

func (repository *ConversationRepository) findMessagesByConversationID(ctx context.Context, conversationID int) ([]conversation.Message, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, conversation_id, sender_role, content, created_on
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_on ASC, id ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding conversation messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	messages := []conversation.Message{}
	for rows.Next() {
		var message conversation.Message
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.SenderRole, &message.Content, &message.CreatedOn); err != nil {
			return nil, fmt.Errorf("scanning conversation message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating conversation messages: %w", err)
	}

	return messages, nil
}

func (repository *ConversationRepository) SaveStatus(ctx context.Context, conversationToSave conversation.Conversation) error {
	base := conversationToSave.Base()
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE conversations
		SET status = $2, updated_on = NOW()
		WHERE id = $1`,
		base.ID,
		base.Status,
	)

	if err != nil {
		return fmt.Errorf("updating conversation status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated conversation rows: %w", err)
	}
	if rowsAffected == 0 {
		return conversation.ErrConversationDoesNotExist
	}

	return nil
}

func (repository *ConversationRepository) saveWorkConversationWithTx(ctx context.Context, tx *sql.Tx, conversationToSave *conversation.WorkConversation) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO work_conversations (conversation_id, consumer_id, provider_id)
		VALUES ($1, $2, $3)`,
		conversationToSave.Base().ID,
		conversationToSave.ConsumerID,
		conversationToSave.ProviderID,
	)
	if err != nil {
		return mapConversationInsertError(err)
	}

	return nil
}

func (repository *ConversationRepository) saveChatbotConversationWithTx(ctx context.Context, tx *sql.Tx, conversationToSave *conversation.ChatBotConversation) error {
	processingStartedOn := sql.NullTime{}
	if startedOn := conversationToSave.ProcessingStartedAt(); startedOn != nil {
		processingStartedOn = sql.NullTime{Time: *startedOn, Valid: true}
	}

	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO chatbot_conversations (conversation_id, consumer_id, title, context_summary, last_summarized_message_id, processing_started_on)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		conversationToSave.Base().ID,
		conversationToSave.ConsumerID,
		conversationToSave.Title,
		conversationToSave.Context.Summary,
		conversationToSave.Context.LastSummarizedMessageID,
		processingStartedOn,
	)
	if err != nil {
		return mapConversationInsertError(err)
	}

	return nil
}

func (repository *ConversationRepository) updateTimestampWithTx(ctx context.Context, tx *sql.Tx, conversationID int, updatedOn time.Time) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE conversations
		SET updated_on = $2
		WHERE id = $1`,
		conversationID,
		updatedOn,
	)
	if err != nil {
		return fmt.Errorf("updating conversation timestamp after message: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated conversation rows: %w", err)
	}
	if rowsAffected == 0 {
		return conversation.ErrConversationDoesNotExist
	}

	return nil
}

func lockConversationByID(ctx context.Context, tx *sql.Tx, conversationID int) error {
	var id int
	err := tx.QueryRowContext(
		ctx,
		`SELECT id
		FROM conversations
		WHERE id = $1
		FOR UPDATE`,
		conversationID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.ErrConversationDoesNotExist
	}
	if err != nil {
		return fmt.Errorf("locking conversation by id: %w", err)
	}

	return nil
}

func rollbackConversationTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback conversation transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}

func mapConversationInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return conversation.ErrAlreadyExists
	}

	return fmt.Errorf("saving conversation: %w", err)
}
