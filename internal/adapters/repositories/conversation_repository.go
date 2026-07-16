package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
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
	var conversationID int
	var updatedOn time.Time
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO conversations (type, status, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, updated_on`,
		conversationToSave.ConversationType(),
		conversationToSave.Status(),
	).Scan(
		&conversationID,
		&updatedOn,
	)
	if err != nil {
		return mapConversationInsertError(err)
	}
	conversationToSave.SetPersistenceState(conversationID, updatedOn)

	messages := conversationToSave.Messages()
	baseMessages := make([]conversation.Message, 0, len(messages))
	for _, message := range messages {
		savedMessage, err := repository.messageRepository.saveWithTx(ctx, tx, conversationID, message)
		if err != nil {
			return err
		}
		baseMessages = append(baseMessages, *savedMessage)
	}
	conversationToSave.SetMessages(baseMessages)

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
	if conversationToUpdate.ID() <= 0 {
		return nil, conversation.ErrConversationDoesNotExist
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning update conversation transaction: %w", err)
	}

	if err := lockConversationByID(ctx, tx, conversationToUpdate.ID()); err != nil {
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
		conversationToUpdate.SetMessages(savedMessages)
		if err := repository.updateTimestampWithTx(ctx, tx, conversationToUpdate.ID(), lastSavedMessage.CreatedOn); err != nil {
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
	result, err := tx.ExecContext(
		ctx,
		`UPDATE conversations
		SET status = $2
		WHERE id = $1`,
		conversationToUpdate.ID(),
		conversationToUpdate.Status(),
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
	conversationID := conversationToUpdate.ID()
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

	if err := repository.saveCurrentAssessmentWithTx(ctx, tx, chatbotConversation); err != nil {
		return err
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE chatbot_conversations
		SET context_summary = $2,
			last_summarized_message_id = $3,
			processing_started_on = NULL,
			last_response_status = $4,
			current_assessment_id = $5
		WHERE conversation_id = $1`,
		chatbotConversation.ID(),
		chatbotConversation.Context.Summary,
		chatbotConversation.Context.LastSummarizedMessageID,
		chatbotConversation.LastResponseStatus,
		optionalAssessmentID(chatbotConversation.CurrentAssessment),
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
		chatbotConversation.ID(),
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
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM chatbot_conversations WHERE conversation_id = $1)`, chatbotConversation.ID()).Scan(&exists); err != nil {
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
	var conversationID int
	var status string
	var updatedOn time.Time
	err := repository.db.QueryRow(
		`SELECT c.id, wc.consumer_id, wc.provider_id, c.status, c.updated_on
		FROM conversations c
		INNER JOIN work_conversations wc ON wc.conversation_id = c.id
		WHERE wc.consumer_id = $1 AND wc.provider_id = $2`,
		consumerID,
		providerID,
	).Scan(
		&conversationID,
		&consumerID,
		&providerID,
		&status,
		&updatedOn,
	)
	if err != nil {
		return nil, fmt.Errorf("finding conversation between consumer and provider: %w", err)
	}

	return &conversation.WorkConversation{
		BaseConversation: conversation.RehydrateBaseConversation(conversationID, conversation.TypeWork, status, updatedOn, nil),
		ConsumerID:       consumerID,
		ProviderID:       providerID,
	}, nil
}

func (repository *ConversationRepository) FindByID(ctx context.Context, conversationID int) (conversation.Conversation, error) {
	var foundConversationID int
	var status string
	var updatedOn time.Time
	var consumerID int
	var providerID int
	var title sql.NullString
	var contextSummary sql.NullString
	var lastSummarizedMessageID sql.NullInt64
	var processingStartedOn sql.NullTime
	var lastResponseStatus sql.NullString
	var assessmentID sql.NullInt64
	var assessmentVersion sql.NullInt64
	var assessmentOutcome sql.NullString
	var assessmentCategoryID sql.NullInt64
	var assessmentTitle sql.NullString
	var assessmentDescription sql.NullString
	var assessmentBasedOnMessageID sql.NullInt64
	var conversationType string
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT
			c.id,
			COALESCE(wc.consumer_id, cc.consumer_id),
			COALESCE(wc.provider_id, 0),
			cc.title,
			cc.context_summary,
			cc.last_summarized_message_id,
			cc.processing_started_on,
			cc.last_response_status,
			pa.id,
			pa.version,
			pa.outcome,
			pa.problem_category_id,
			pa.problem_title,
			pa.problem_description,
			pa.based_on_message_id,
			c.status,
			c.updated_on,
			c.type
		FROM conversations c
		LEFT JOIN work_conversations wc ON wc.conversation_id = c.id
		LEFT JOIN chatbot_conversations cc ON cc.conversation_id = c.id
		LEFT JOIN problem_assessments pa ON pa.id = cc.current_assessment_id
		WHERE c.id = $1`,
		conversationID,
	).Scan(
		&foundConversationID,
		&consumerID,
		&providerID,
		&title,
		&contextSummary,
		&lastSummarizedMessageID,
		&processingStartedOn,
		&lastResponseStatus,
		&assessmentID,
		&assessmentVersion,
		&assessmentOutcome,
		&assessmentCategoryID,
		&assessmentTitle,
		&assessmentDescription,
		&assessmentBasedOnMessageID,
		&status,
		&updatedOn,
		&conversationType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, conversation.ErrConversationDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding conversation by id: %w", err)
	}

	messages, err := repository.findMessagesByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	base := conversation.RehydrateBaseConversation(foundConversationID, conversationType, status, updatedOn, messages)

	switch conversationType {
	case conversation.TypeChatbot:
		var processingStartedAt *time.Time
		if processingStartedOn.Valid {
			processingStartedAt = &processingStartedOn.Time
		}
		responseStatus, err := conversation.ParseChatbotResponseStatus(lastResponseStatus.String)
		if err != nil {
			return nil, err
		}
		chatbotConversation := &conversation.ChatBotConversation{
			BaseConversation:   base,
			ConsumerID:         consumerID,
			Title:              title.String,
			LastResponseStatus: responseStatus,
			Context: conversation.ChatbotConversationContext{
				Summary:                 contextSummary.String,
				LastSummarizedMessageID: int(lastSummarizedMessageID.Int64),
			},
			ProcessingStartedOn: processingStartedAt,
		}
		if assessmentID.Valid {
			outcome, err := conversation.ParseProblemAssessmentOutcome(assessmentOutcome.String)
			if err != nil {
				return nil, err
			}
			chatbotConversation.CurrentAssessment = &conversation.ProblemAssessment{
				ID:                    int(assessmentID.Int64),
				ChatbotConversationID: base.ID(),
				Version:               int(assessmentVersion.Int64),
				Outcome:               outcome,
				ProblemCategoryID:     optionalIntFromSQLNullInt64(assessmentCategoryID),
				ProblemTitle:          assessmentTitle.String,
				ProblemDescription:    assessmentDescription.String,
				BasedOnMessageID:      int(assessmentBasedOnMessageID.Int64),
			}
			chatbotConversation.CurrentAssessment.Images, err = repository.findAssessmentImages(ctx, chatbotConversation.CurrentAssessment.ID)
			if err != nil {
				return nil, err
			}
		}
		return chatbotConversation, nil
	case conversation.TypeWork:
		return &conversation.WorkConversation{
			BaseConversation: base,
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
	imagesByMessageID, err := repository.messageRepository.findImagesByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	attachImagesToMessages(messages, imagesByMessageID)

	return messages, nil
}

func optionalIntFromSQLNullInt64(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}

	converted := int(value.Int64)
	return &converted
}

func (repository *ConversationRepository) SaveStatus(ctx context.Context, conversationToSave conversation.Conversation) error {
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE conversations
		SET status = $2, updated_on = NOW()
		WHERE id = $1`,
		conversationToSave.ID(),
		conversationToSave.Status(),
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
		conversationToSave.ID(),
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
		`INSERT INTO chatbot_conversations (
			conversation_id,
			consumer_id,
			title,
			context_summary,
			last_summarized_message_id,
			processing_started_on,
			last_response_status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		conversationToSave.ID(),
		conversationToSave.ConsumerID,
		conversationToSave.Title,
		conversationToSave.Context.Summary,
		conversationToSave.Context.LastSummarizedMessageID,
		processingStartedOn,
		conversationToSave.LastResponseStatus,
	)
	if err != nil {
		return mapConversationInsertError(err)
	}

	if err := repository.saveCurrentAssessmentWithTx(ctx, tx, conversationToSave); err != nil {
		return err
	}
	if conversationToSave.CurrentAssessment == nil {
		return nil
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE chatbot_conversations SET current_assessment_id = $2 WHERE conversation_id = $1`,
		conversationToSave.ID(),
		conversationToSave.CurrentAssessment.ID,
	)
	return err
}

func (repository *ConversationRepository) saveCurrentAssessmentWithTx(ctx context.Context, tx *sql.Tx, chatbotConversation *conversation.ChatBotConversation) error {
	assessment := chatbotConversation.CurrentAssessment
	if assessment == nil || assessment.ID > 0 {
		return nil
	}
	messages := chatbotConversation.Messages()

	assessment.ChatbotConversationID = chatbotConversation.ID()
	assessment.BasedOnMessageID = messages[len(messages)-1].ID
	if err := assessment.Validate(); err != nil {
		return err
	}

	var categoryID sql.NullInt64
	if assessment.ProblemCategoryID != nil {
		categoryID = sql.NullInt64{Int64: int64(*assessment.ProblemCategoryID), Valid: true}
	}
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO problem_assessments (
			chatbot_conversation_id, version, outcome, problem_category_id,
			problem_title, problem_description, based_on_message_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		assessment.ChatbotConversationID,
		assessment.Version,
		assessment.Outcome,
		categoryID,
		assessment.ProblemTitle,
		assessment.ProblemDescription,
		assessment.BasedOnMessageID,
	).Scan(&assessment.ID); err != nil {
		return err
	}
	for position, image := range assessment.Images {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO problem_assessment_images (problem_assessment_id, file_id, position)
			 VALUES ($1, $2, $3)`,
			assessment.ID, image.FileID, position,
		); err != nil {
			return fmt.Errorf("saving problem assessment image: %w", err)
		}
	}
	return nil
}

func (repository *ConversationRepository) findAssessmentImages(ctx context.Context, assessmentID int) ([]filedomain.MessageImage, error) {
	rows, err := repository.db.QueryContext(ctx,
		`SELECT pai.file_id::text, f.original_name, mi.description
		 FROM problem_assessment_images pai
		 INNER JOIN files f ON f.id = pai.file_id
		 INNER JOIN message_images mi ON mi.file_id = pai.file_id
		 WHERE pai.problem_assessment_id = $1
		 ORDER BY pai.position`,
		assessmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding problem assessment images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	images := []filedomain.MessageImage{}
	for rows.Next() {
		var image filedomain.MessageImage
		if err := rows.Scan(&image.FileID, &image.OriginalName, &image.Description); err != nil {
			return nil, fmt.Errorf("scanning problem assessment image: %w", err)
		}
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating problem assessment images: %w", err)
	}
	return images, nil
}

func optionalAssessmentID(assessment *conversation.ProblemAssessment) any {
	if assessment == nil {
		return nil
	}
	return assessment.ID
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
