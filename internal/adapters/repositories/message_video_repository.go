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

type MessageVideoRepository struct {
	db *sql.DB
}

func NewMessageVideoRepository(db *sql.DB) *MessageVideoRepository {
	return &MessageVideoRepository{db: db}
}

func (repository *MessageVideoRepository) saveWithTx(ctx context.Context, tx *sql.Tx, messageID int, video *filedomain.MessageVideo) error {
	if video == nil {
		return nil
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO message_videos (message_id, file_id) VALUES ($1, $2)`,
		messageID, video.FileID,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == uniqueViolationCode || pgErr.Code == foreignKeyViolationCode) {
			return conversation.ErrMessageVideoNotAvailable
		}
		return fmt.Errorf("saving message video: %w", err)
	}
	return nil
}

type persistedMessageVideo struct {
	MessageID       int
	FileID          string
	OriginalName    string
	MimeType        string
	VideoCodec      string
	AudioCodec      string
	DurationSeconds int
	Width           int
	Height          int
}

func (repository *MessageVideoRepository) findByConversationID(ctx context.Context, conversationID int) (map[int]persistedMessageVideo, error) {
	rows, err := repository.db.QueryContext(ctx,
		`SELECT mv.message_id, mv.file_id::text, f.original_name, f.mime_type,
			fv.video_codec, COALESCE(fv.audio_codec, ''), fv.duration_seconds, fv.width, fv.height
		 FROM message_videos mv
		 INNER JOIN messages m ON m.id = mv.message_id
		 INNER JOIN files f ON f.id = mv.file_id
		 INNER JOIN file_videos fv ON fv.file_id = mv.file_id
		 WHERE m.conversation_id = $1`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding message videos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	videosByMessageID := map[int]persistedMessageVideo{}
	for rows.Next() {
		var video persistedMessageVideo
		if err := rows.Scan(
			&video.MessageID,
			&video.FileID,
			&video.OriginalName,
			&video.MimeType,
			&video.VideoCodec,
			&video.AudioCodec,
			&video.DurationSeconds,
			&video.Width,
			&video.Height,
		); err != nil {
			return nil, fmt.Errorf("scanning message video: %w", err)
		}
		videosByMessageID[video.MessageID] = video
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating message videos: %w", err)
	}
	return videosByMessageID, nil
}

func attachVideosToMessages(messages []conversation.Message, videosByMessageID map[int]persistedMessageVideo) {
	for index := range messages {
		video, ok := videosByMessageID[messages[index].ID]
		if !ok {
			continue
		}
		messages[index].Video = messageVideoFromPersistence(video)
	}
}

func attachVideosToMessageDetails(messages []readmodel.MessageDetail, videosByMessageID map[int]persistedMessageVideo) {
	for index := range messages {
		video, ok := videosByMessageID[messages[index].ID]
		if !ok {
			continue
		}
		messages[index].Video = messageVideoFromPersistence(video)
	}
}

func messageVideoFromPersistence(video persistedMessageVideo) *filedomain.MessageVideo {
	return &filedomain.MessageVideo{
		FileID:          video.FileID,
		OriginalName:    video.OriginalName,
		MimeType:        video.MimeType,
		VideoCodec:      video.VideoCodec,
		AudioCodec:      video.AudioCodec,
		DurationSeconds: video.DurationSeconds,
		Width:           video.Width,
		Height:          video.Height,
	}
}
