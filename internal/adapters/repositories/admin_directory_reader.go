package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
)

type AdminDirectoryReader struct {
	db *sql.DB
}

func NewAdminDirectoryReader(database *sql.DB) *AdminDirectoryReader {
	return &AdminDirectoryReader{db: database}
}

func (reader *AdminDirectoryReader) FindConsumers(ctx context.Context) ([]readmodel.Consumer, error) {
	rows, err := reader.db.QueryContext(
		ctx,
		`SELECT users.id, users.name, users.surname, users.email,
			COALESCE(users.profile_photo_file_id::text, ''), users.created_on
		FROM consumers
		INNER JOIN users ON users.id = consumers.user_id
		WHERE users.role = $1
		ORDER BY users.created_on ASC, users.id ASC`,
		consumer.Role,
	)
	if err != nil {
		return nil, fmt.Errorf("querying administrative consumer directory: %w", err)
	}
	defer rows.Close()

	consumers := make([]readmodel.Consumer, 0)
	for rows.Next() {
		var found readmodel.Consumer
		if err := rows.Scan(
			&found.ID,
			&found.Name,
			&found.Surname,
			&found.Email,
			&found.ProfilePhotoFileID,
			&found.CreatedOn,
		); err != nil {
			return nil, fmt.Errorf("scanning administrative consumer directory: %w", err)
		}
		consumers = append(consumers, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating administrative consumer directory: %w", err)
	}

	return consumers, nil
}
