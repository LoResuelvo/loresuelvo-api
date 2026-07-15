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

type MessageImageRepository struct {
	db *sql.DB
}

func NewMessageImageRepository(db *sql.DB) *MessageImageRepository {
	return &MessageImageRepository{db: db}
}

func (repository *MessageImageRepository) saveWithTx(ctx context.Context, tx *sql.Tx, messageID int, images []filedomain.MessageImage) error {
	for position, image := range images {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_images (message_id, file_id, position, description) VALUES ($1, $2, $3, $4)`,
			messageID, image.FileID, position, image.Description,
		); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
				return conversation.ErrMessageImageNotAvailable
			}
			return fmt.Errorf("saving message image: %w", err)
		}
	}
	return nil
}

type persistedMessageImage struct {
	MessageID    int
	FileID       string
	OriginalName string
	Description  string
}

func (repository *MessageImageRepository) findByConversationID(ctx context.Context, conversationID int) (map[int][]persistedMessageImage, error) {
	rows, err := repository.db.QueryContext(ctx,
		`SELECT mi.message_id, mi.file_id::text, f.original_name, mi.description
		 FROM message_images mi
		 INNER JOIN messages m ON m.id = mi.message_id
		 INNER JOIN files f ON f.id = mi.file_id
		 WHERE m.conversation_id = $1
		 ORDER BY mi.message_id, mi.position`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding message images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	imagesByMessageID := map[int][]persistedMessageImage{}
	for rows.Next() {
		var image persistedMessageImage
		if err := rows.Scan(&image.MessageID, &image.FileID, &image.OriginalName, &image.Description); err != nil {
			return nil, fmt.Errorf("scanning message image: %w", err)
		}
		imagesByMessageID[image.MessageID] = append(imagesByMessageID[image.MessageID], image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating message images: %w", err)
	}
	return imagesByMessageID, nil
}

func attachImagesToMessages(messages []conversation.Message, imagesByMessageID map[int][]persistedMessageImage) {
	for index := range messages {
		persistedImages := imagesByMessageID[messages[index].ID]
		messages[index].Images = make([]filedomain.MessageImage, 0, len(persistedImages))
		for _, image := range persistedImages {
			messages[index].Images = append(messages[index].Images, filedomain.MessageImage{Image: filedomain.Image{FileID: image.FileID, OriginalName: image.OriginalName}, Description: image.Description})
		}
	}
}

func attachImagesToMessageDetails(messages []readmodel.MessageDetail, imagesByMessageID map[int][]persistedMessageImage) {
	for index := range messages {
		persistedImages := imagesByMessageID[messages[index].ID]
		messages[index].Images = make([]filedomain.MessageImage, 0, len(persistedImages))
		for _, image := range persistedImages {
			messages[index].Images = append(messages[index].Images, filedomain.MessageImage{Image: filedomain.Image{FileID: image.FileID, OriginalName: image.OriginalName}, Description: image.Description})
		}
	}
}
