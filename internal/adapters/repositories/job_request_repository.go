package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request/read_model"
	"github.com/jackc/pgx/v5/pgconn"
)

type JobRequestRepository struct {
	db *sql.DB
}

func NewJobRequestRepository(db *sql.DB) *JobRequestRepository {
	return &JobRequestRepository{db: db}
}

// TODO: reemplazar inserción de Conversación y remover ids de consumer y provider de JobRequest para tener como fuente de verdad a Conversation
func (repository *JobRequestRepository) SaveWithConversation(jobRequest jobrequest.JobRequest, pendingConversation conversation.Conversation) (*jobrequest.JobRequest, error) {
	workConversation, ok := pendingConversation.(*conversation.WorkConversation)
	if !ok {
		return nil, fmt.Errorf("saving job request: expected work conversation, got %s", pendingConversation.ConversationType())
	}

	ctx := context.Background()
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning job request transaction: %w", err)
	}

	var conversationID int
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO conversations (type, status, created_on, updated_on)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id`,
		workConversation.ConversationType(),
		workConversation.Status,
	).Scan(&conversationID)
	if err != nil {
		return nil, rollbackJobRequestTx(tx, mapJobRequestInsertError(err))
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO work_conversations (conversation_id, consumer_id, provider_id)
		VALUES ($1, $2, $3)`,
		conversationID,
		workConversation.ConsumerID,
		workConversation.ProviderID,
	)
	if err != nil {
		return nil, rollbackJobRequestTx(tx, mapJobRequestInsertError(err))
	}

	var savedJobRequest jobrequest.JobRequest
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO job_requests (consumer_id, provider_id, conversation_id, title, description, status, source_assessment_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id, consumer_id, provider_id, conversation_id, title, description, status, source_assessment_id`,
		jobRequest.ConsumerID,
		jobRequest.ProviderID,
		conversationID,
		jobRequest.Title,
		jobRequest.Description,
		jobRequest.Status,
		jobRequest.SourceAssessmentID,
	).Scan(
		&savedJobRequest.ID,
		&savedJobRequest.ConsumerID,
		&savedJobRequest.ProviderID,
		&savedJobRequest.ConversationID,
		&savedJobRequest.Title,
		&savedJobRequest.Description,
		&savedJobRequest.Status,
		&savedJobRequest.SourceAssessmentID,
	)
	if err != nil {
		return nil, rollbackJobRequestTx(tx, mapJobRequestInsertError(err))
	}
	savedJobRequest.Images = append([]filedomain.MessageImage(nil), jobRequest.Images...)

	if err := saveJobRequestImagesWithTx(ctx, tx, savedJobRequest.ID, savedJobRequest.Images); err != nil {
		return nil, rollbackJobRequestTx(tx, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing job request transaction: %w", err)
	}

	return &savedJobRequest, nil
}

func (repository *JobRequestRepository) ExistsBetweenWithAnyStatus(consumerID, providerID int, statuses []jobrequest.Status) (bool, error) {
	if len(statuses) == 0 {
		return false, nil
	}

	statusValues := make([]string, len(statuses))
	for i, status := range statuses {
		statusValues[i] = string(status)
	}

	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS (
			SELECT 1
			FROM job_requests
			WHERE consumer_id = $1
				AND provider_id = $2
				AND status = ANY($3)
		)`,
		consumerID,
		providerID,
		statusValues,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking job request between consumer and provider with any status: %w", err)
	}

	return exists, nil
}

func (repository *JobRequestRepository) FindByConversationID(conversationID int) (*jobrequest.JobRequest, error) {
	var foundJobRequest jobrequest.JobRequest
	err := repository.db.QueryRow(
		`SELECT id, consumer_id, provider_id, conversation_id, title, description, status, source_assessment_id
		FROM job_requests
		WHERE conversation_id = $1`,
		conversationID,
	).Scan(
		&foundJobRequest.ID,
		&foundJobRequest.ConsumerID,
		&foundJobRequest.ProviderID,
		&foundJobRequest.ConversationID,
		&foundJobRequest.Title,
		&foundJobRequest.Description,
		&foundJobRequest.Status,
		&foundJobRequest.SourceAssessmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding job request by conversation id: %w", err)
	}
	imagesByJobRequestID, err := repository.findImagesByJobRequestIDs([]int{foundJobRequest.ID})
	if err != nil {
		return nil, err
	}
	foundJobRequest.Images = domainImagesFromReadModel(imagesByJobRequestID[foundJobRequest.ID])

	return &foundJobRequest, nil
}

func (repository *JobRequestRepository) FindByID(id int) (*jobrequest.JobRequest, error) {
	var foundJobRequest jobrequest.JobRequest
	err := repository.db.QueryRow(
		`SELECT id, consumer_id, provider_id, conversation_id, title, description, status, source_assessment_id
		FROM job_requests
		WHERE id = $1`,
		id,
	).Scan(
		&foundJobRequest.ID,
		&foundJobRequest.ConsumerID,
		&foundJobRequest.ProviderID,
		&foundJobRequest.ConversationID,
		&foundJobRequest.Title,
		&foundJobRequest.Description,
		&foundJobRequest.Status,
		&foundJobRequest.SourceAssessmentID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, jobrequest.ErrJobRequestNotFound
		}
		return nil, fmt.Errorf("finding job request by id: %w", err)
	}
	imagesByJobRequestID, err := repository.findImagesByJobRequestIDs([]int{foundJobRequest.ID})
	if err != nil {
		return nil, err
	}
	foundJobRequest.Images = domainImagesFromReadModel(imagesByJobRequestID[foundJobRequest.ID])

	return &foundJobRequest, nil
}

func (repository *JobRequestRepository) SaveStatus(ctx context.Context, jobRequest jobrequest.JobRequest) error {
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE job_requests
		SET status = $2, updated_on = NOW()
		WHERE id = $1`,
		jobRequest.ID,
		jobRequest.Status,
	)
	if err != nil {
		return fmt.Errorf("saving job request status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking saved job request status: %w", err)
	}
	if rowsAffected == 0 {
		return jobrequest.ErrJobRequestNotFound
	}

	return nil
}

func (repository *JobRequestRepository) FindByUserAuthID(userAuthID string) ([]readmodel.JobRequestSummary, error) {
	rows, err := repository.db.Query(
		`SELECT job_requests.id,
			job_requests.conversation_id,
			job_requests.title,
			job_requests.description,
			job_requests.status,
			consumer_users.name,
			consumer_users.surname
		FROM job_requests
		INNER JOIN conversations ON conversations.id = job_requests.conversation_id
		INNER JOIN consumers ON consumers.id = job_requests.consumer_id
		INNER JOIN users AS consumer_users ON consumer_users.id = consumers.user_id
		INNER JOIN providers ON providers.id = job_requests.provider_id
		INNER JOIN users AS provider_users ON provider_users.id = providers.user_id
		WHERE job_requests.status = $1
			AND (consumer_users.auth_id = $2 OR provider_users.auth_id = $2)
		ORDER BY job_requests.created_on DESC, job_requests.id DESC`,
		jobrequest.StatusPending,
		userAuthID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding job requests by user auth id: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	jobRequests := []readmodel.JobRequestSummary{}
	for rows.Next() {
		var foundJobRequest readmodel.JobRequestSummary
		if err := rows.Scan(
			&foundJobRequest.ID,
			&foundJobRequest.ConversationID,
			&foundJobRequest.Title,
			&foundJobRequest.Description,
			&foundJobRequest.Status,
			&foundJobRequest.Requester.Name,
			&foundJobRequest.Requester.Surname,
		); err != nil {
			return nil, fmt.Errorf("scanning job request by user auth id: %w", err)
		}

		jobRequests = append(jobRequests, foundJobRequest)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating job requests by user auth id: %w", err)
	}

	jobRequestIDs := make([]int, 0, len(jobRequests))
	for _, jobRequest := range jobRequests {
		jobRequestIDs = append(jobRequestIDs, jobRequest.ID)
	}
	imagesByJobRequestID, err := repository.findImagesByJobRequestIDs(jobRequestIDs)
	if err != nil {
		return nil, err
	}
	for index := range jobRequests {
		jobRequests[index].Images = imagesByJobRequestID[jobRequests[index].ID]
	}

	return jobRequests, nil
}

func saveJobRequestImagesWithTx(ctx context.Context, tx *sql.Tx, jobRequestID int, images []filedomain.MessageImage) error {
	for position, image := range images {
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO job_request_images (job_request_id, file_id, position)
			VALUES ($1, $2, $3)`,
			jobRequestID,
			image.FileID,
			position,
		)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
				return jobrequest.ErrJobRequestImageNotAvailable
			}
			return fmt.Errorf("saving job request image: %w", err)
		}
	}

	return nil
}

func (repository *JobRequestRepository) findImagesByJobRequestIDs(jobRequestIDs []int) (map[int][]readmodel.JobRequestImage, error) {
	imagesByJobRequestID := make(map[int][]readmodel.JobRequestImage, len(jobRequestIDs))
	if len(jobRequestIDs) == 0 {
		return imagesByJobRequestID, nil
	}

	args := make([]any, len(jobRequestIDs))
	placeholders := make([]string, len(jobRequestIDs))
	for index, jobRequestID := range jobRequestIDs {
		args[index] = jobRequestID
		placeholders[index] = fmt.Sprintf("$%d", index+1)
	}

	rows, err := repository.db.Query(
		fmt.Sprintf(
			`SELECT job_request_images.job_request_id,
				job_request_images.file_id::text,
				files.original_name
			FROM job_request_images
			INNER JOIN files ON files.id = job_request_images.file_id
			WHERE job_request_images.job_request_id IN (%s)
			ORDER BY job_request_images.job_request_id, job_request_images.position`,
			strings.Join(placeholders, ", "),
		),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("finding job request images: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var jobRequestID int
		var image readmodel.JobRequestImage
		if err := rows.Scan(&jobRequestID, &image.FileID, &image.OriginalName); err != nil {
			return nil, fmt.Errorf("scanning job request image: %w", err)
		}
		imagesByJobRequestID[jobRequestID] = append(imagesByJobRequestID[jobRequestID], image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating job request images: %w", err)
	}

	return imagesByJobRequestID, nil
}

func domainImagesFromReadModel(images []readmodel.JobRequestImage) []filedomain.MessageImage {
	if len(images) == 0 {
		return []filedomain.MessageImage{}
	}
	result := make([]filedomain.MessageImage, 0, len(images))
	for _, image := range images {
		result = append(result, filedomain.MessageImage{
			FileID:       image.FileID,
			OriginalName: image.OriginalName,
			URL:          image.URL,
		})
	}
	return result
}

func (repository *JobRequestRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM job_requests`)
	if err != nil {
		return fmt.Errorf("deleting all job requests: %w", err)
	}

	return nil
}

func rollbackJobRequestTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback job request transaction: %v", originalErr, rollbackErr)
	}

	return originalErr
}

func mapJobRequestInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return jobrequest.ErrAlreadyExists
	}

	return fmt.Errorf("saving job request: %w", err)
}
