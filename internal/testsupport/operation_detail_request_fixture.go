package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OperationDetailRequestFixture persists historical evidence whose timestamps and
// foreign keys cannot be set through the public request-creation API.
type OperationDetailRequestFixture struct{ DB *sql.DB }

func (fixture OperationDetailRequestFixture) SetConsumerAddress(ctx context.Context, consumerID int, street, number, floor, unit string) error {
	result, err := fixture.DB.ExecContext(ctx, `UPDATE consumer_addresses SET street=$2, street_number=$3, floor=$4, unit=$5 WHERE consumer_id=$1`, consumerID, street, number, floor, unit)
	if err != nil {
		return fmt.Errorf("setting consumer address: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("consumer %d has no persisted address", consumerID)
	}
	return nil
}

func (fixture OperationDetailRequestFixture) CreateChatbotConversation(ctx context.Context, consumerID int, created time.Time) (int, error) {
	tx, err := fixture.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var id int
	err = tx.QueryRowContext(ctx, `INSERT INTO conversations (type,status,created_on,updated_on) VALUES ('chatbot','active',$1,$1) RETURNING id`, created).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("creating chatbot conversation: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO chatbot_conversations (conversation_id,consumer_id,title,context_summary,last_summarized_message_id,last_response_status)
		VALUES ($1,$2,'Historical assessment conversation','',0,'answered')`, id, consumerID)
	if err != nil {
		return 0, fmt.Errorf("linking chatbot conversation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (fixture OperationDetailRequestFixture) AddMessage(ctx context.Context, conversationID int, content string, created time.Time) (int, error) {
	var id int
	err := fixture.DB.QueryRowContext(ctx, `INSERT INTO messages (conversation_id,sender_role,content,created_on) VALUES ($1,'consumer',$2,$3) RETURNING id`, conversationID, content, created).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("creating historical message: %w", err)
	}
	return id, nil
}

func (fixture OperationDetailRequestFixture) AddAssessment(ctx context.Context, conversationID, version, messageID int, outcome string, categoryID *int, title, description string, created time.Time) (int, error) {
	var id int
	err := fixture.DB.QueryRowContext(ctx, `INSERT INTO problem_assessments (chatbot_conversation_id,version,outcome,problem_category_id,problem_title,problem_description,based_on_message_id,created_on) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, conversationID, version, outcome, categoryID, title, description, messageID, created).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("creating historical assessment: %w", err)
	}
	_, err = fixture.DB.ExecContext(ctx, `UPDATE chatbot_conversations SET current_assessment_id=$2 WHERE conversation_id=$1`, conversationID, id)
	if err != nil {
		return 0, fmt.Errorf("setting current assessment: %w", err)
	}
	return id, nil
}

func (fixture OperationDetailRequestFixture) SetRequestAssessment(ctx context.Context, requestID, assessmentID int) error {
	_, err := fixture.DB.ExecContext(ctx, `UPDATE job_requests SET source_assessment_id=$2 WHERE id=$1`, requestID, assessmentID)
	if err != nil {
		return fmt.Errorf("linking source assessment: %w", err)
	}
	return nil
}

func (fixture OperationDetailRequestFixture) AddRequestImage(ctx context.Context, requestID int, fileID string) error {
	_, err := fixture.DB.ExecContext(ctx, `INSERT INTO job_request_images (job_request_id,file_id,position) VALUES ($1,$2,0)`, requestID, fileID)
	if err != nil {
		return fmt.Errorf("linking request image: %w", err)
	}
	return nil
}
