package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/jackc/pgx/v5/pgconn"
)

type MessageAudioRepository struct {
	db *sql.DB
}

func NewMessageAudioRepository(db *sql.DB) *MessageAudioRepository {
	return &MessageAudioRepository{db: db}
}

func (repository *MessageAudioRepository) saveWithTx(ctx context.Context, tx *sql.Tx, messageID int, audio *filedomain.MessageAudio) error {
	if audio == nil {
		return nil
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO message_audios (message_id, file_id) VALUES ($1, $2)`,
		messageID, audio.FileID,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == uniqueViolationCode || pgErr.Code == foreignKeyViolationCode) {
			return conversation.ErrMessageAudioNotAvailable
		}
		return fmt.Errorf("saving message audio: %w", err)
	}
	return nil
}

type persistedMessageAudio struct {
	MessageID       int
	FileID          string
	OriginalName    string
	MimeType        string
	Codec           string
	DurationSeconds int
}

func (repository *MessageAudioRepository) findByConversationID(ctx context.Context, conversationID int) (map[int]persistedMessageAudio, error) {
	rows, err := repository.db.QueryContext(ctx,
		`SELECT ma.message_id, ma.file_id::text, f.original_name, f.mime_type, fa.codec, fa.duration_seconds
		 FROM message_audios ma
		 INNER JOIN messages m ON m.id = ma.message_id
		 INNER JOIN files f ON f.id = ma.file_id
		 INNER JOIN file_audios fa ON fa.file_id = ma.file_id
		 WHERE m.conversation_id = $1`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding message audios: %w", err)
	}
	defer func() { _ = rows.Close() }()

	audiosByMessageID := map[int]persistedMessageAudio{}
	for rows.Next() {
		var audio persistedMessageAudio
		if err := rows.Scan(
			&audio.MessageID,
			&audio.FileID,
			&audio.OriginalName,
			&audio.MimeType,
			&audio.Codec,
			&audio.DurationSeconds,
		); err != nil {
			return nil, fmt.Errorf("scanning message audio: %w", err)
		}
		audiosByMessageID[audio.MessageID] = audio
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating message audios: %w", err)
	}
	return audiosByMessageID, nil
}

func attachAudiosToMessages(messages []conversation.Message, audiosByMessageID map[int]persistedMessageAudio) {
	for index := range messages {
		audio, ok := audiosByMessageID[messages[index].ID]
		if !ok {
			continue
		}
		messages[index].Audio = messageAudioFromPersistence(audio)
	}
}

func attachAudiosToMessageDetails(messages []readmodel.MessageDetail, audiosByMessageID map[int]persistedMessageAudio) {
	for index := range messages {
		audio, ok := audiosByMessageID[messages[index].ID]
		if !ok {
			continue
		}
		messages[index].Audio = messageAudioFromPersistence(audio)
	}
}

func messageAudioFromPersistence(audio persistedMessageAudio) *filedomain.MessageAudio {
	return &filedomain.MessageAudio{
		FileID:          audio.FileID,
		OriginalName:    audio.OriginalName,
		MimeType:        audio.MimeType,
		Codec:           audio.Codec,
		DurationSeconds: audio.DurationSeconds,
	}
}

func (repository *MessageAudioRepository) FindByMessagePage(ctx context.Context, scope conversation.MessageAttachmentScope) (map[int]string, error) {
	if err := validateMessageAttachmentScope(scope); err != nil {
		return nil, err
	}
	result := make(map[int]string)
	if len(scope.MessageIDs) == 0 {
		return result, nil
	}
	rows, err := repository.db.QueryContext(ctx, `SELECT attachment.message_id, attachment.file_id::text FROM message_audios attachment`+messageAttachmentOwnershipSQL, scope.ConversationID, scope.ConsumerID, scope.ProviderID, scope.MessageIDs)
	if err != nil {
		return nil, fmt.Errorf("querying page message audios: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var fileID string
		if err := rows.Scan(&id, &fileID); err != nil {
			return nil, fmt.Errorf("scanning page message audio: %w", err)
		}
		result[id] = fileID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating page message audios: %w", err)
	}
	return result, nil
}
