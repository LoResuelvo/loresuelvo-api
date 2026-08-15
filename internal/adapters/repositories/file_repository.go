package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type FileRepository struct {
	db *sql.DB
}

func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (repository *FileRepository) Save(ctx context.Context, file filedomain.File) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning file transaction: %w", err)
	}

	if err := repository.saveFileWithTx(ctx, tx, file); err != nil {
		return rollbackFileTx(tx, err)
	}
	if err := repository.syncAudioMetadataWithTx(ctx, tx, file); err != nil {
		return rollbackFileTx(tx, err)
	}
	if err := repository.syncVideoMetadataWithTx(ctx, tx, file); err != nil {
		return rollbackFileTx(tx, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing file transaction: %w", err)
	}

	return nil
}

func (repository *FileRepository) saveFileWithTx(ctx context.Context, tx *sql.Tx, file filedomain.File) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			key = EXCLUDED.key,
			bucket = EXCLUDED.bucket,
			original_name = EXCLUDED.original_name,
			mime_type = EXCLUDED.mime_type,
			size_bytes = EXCLUDED.size_bytes,
			status = EXCLUDED.status,
			visibility = EXCLUDED.visibility,
			purpose = EXCLUDED.purpose,
			uploaded_by_auth_id = EXCLUDED.uploaded_by_auth_id,
			updated_on = EXCLUDED.updated_on`,
		file.ID,
		file.Key,
		file.Bucket,
		file.OriginalName(),
		file.MimeType(),
		file.SizeBytes(),
		file.Status,
		file.Visibility,
		file.Purpose,
		file.UploadedByAuthID,
		file.CreatedOn,
		file.UpdatedOn,
	)
	if err != nil {
		return fmt.Errorf("saving file: %w", err)
	}

	return nil
}

func (repository *FileRepository) syncAudioMetadataWithTx(ctx context.Context, tx *sql.Tx, file filedomain.File) error {
	if !file.IsAudio() || !file.IsConfirmed() {
		if _, err := tx.ExecContext(ctx, `DELETE FROM file_audios WHERE file_id = $1`, file.ID); err != nil {
			return fmt.Errorf("removing file audio metadata: %w", err)
		}
		return nil
	}

	if file.DurationSeconds() <= 0 || strings.TrimSpace(file.Codec()) == "" {
		return fmt.Errorf("saving file audio metadata: %w", filedomain.ErrMessageAudioNotAvailable)
	}

	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO file_audios (file_id, codec, duration_seconds)
		VALUES ($1, $2, $3)
		ON CONFLICT (file_id) DO UPDATE SET
			codec = EXCLUDED.codec,
			duration_seconds = EXCLUDED.duration_seconds`,
		file.ID,
		strings.ToLower(strings.TrimSpace(file.Codec())),
		file.DurationSeconds(),
	)
	if err != nil {
		return fmt.Errorf("saving file audio metadata: %w", err)
	}

	return nil
}

func (repository *FileRepository) syncVideoMetadataWithTx(ctx context.Context, tx *sql.Tx, file filedomain.File) error {
	if !file.IsVideo() || !file.IsConfirmed() {
		if _, err := tx.ExecContext(ctx, `DELETE FROM file_videos WHERE file_id = $1`, file.ID); err != nil {
			return fmt.Errorf("removing file video metadata: %w", err)
		}
		return nil
	}

	if file.VideoCodec() == "" || file.DurationSeconds() <= 0 || file.Width() <= 0 || file.Height() <= 0 {
		return fmt.Errorf("saving file video metadata: %w", filedomain.ErrMessageVideoNotAvailable)
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO file_videos (file_id, video_codec, audio_codec, duration_seconds, width, height)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6)
		ON CONFLICT (file_id) DO UPDATE SET
			video_codec = EXCLUDED.video_codec,
			audio_codec = EXCLUDED.audio_codec,
			duration_seconds = EXCLUDED.duration_seconds,
			width = EXCLUDED.width,
			height = EXCLUDED.height`,
		file.ID,
		strings.ToLower(strings.TrimSpace(file.VideoCodec())),
		strings.ToLower(strings.TrimSpace(file.AudioCodec())),
		file.DurationSeconds(),
		file.Width(),
		file.Height(),
	)
	if err != nil {
		return fmt.Errorf("saving file video metadata: %w", err)
	}

	return nil
}

func (repository *FileRepository) FindByID(ctx context.Context, id string) (*filedomain.File, error) {
	file, err := repository.findOne(ctx, fileSelectQuery+`WHERE f.id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("finding file by id: %w", err)
	}

	return file, nil
}

func (repository *FileRepository) FindByIDs(ctx context.Context, ids []string) ([]filedomain.File, error) {
	if len(ids) == 0 {
		return []filedomain.File{}, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	rows, err := repository.db.QueryContext(
		ctx,
		fileSelectQuery+`WHERE f.id IN (`+strings.Join(placeholders, ", ")+`)`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("finding files by ids: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	files := make([]filedomain.File, 0, len(ids))
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating files: %w", err)
	}

	return files, nil
}

func (repository *FileRepository) findOne(ctx context.Context, query string, args ...any) (*filedomain.File, error) {
	return scanFile(repository.db.QueryRowContext(ctx, query, args...))
}

const fileSelectQuery = `SELECT f.id, f.key, f.bucket, f.original_name, f.mime_type, f.size_bytes,
	fa.duration_seconds, fa.codec,
	fv.video_codec, fv.audio_codec, fv.duration_seconds, fv.width, fv.height,
	f.status, f.visibility, f.purpose, f.uploaded_by_auth_id, f.created_on, f.updated_on
	FROM files f
	LEFT JOIN file_audios fa ON fa.file_id = f.id
	LEFT JOIN file_videos fv ON fv.file_id = f.id
	`

type fileScanner interface {
	Scan(dest ...any) error
}

func scanFile(scanner fileScanner) (*filedomain.File, error) {
	var id string
	var key string
	var bucket string
	var originalName string
	var mimeType string
	var sizeBytes int
	var durationSeconds sql.NullInt64
	var codec sql.NullString
	var videoCodec sql.NullString
	var audioCodec sql.NullString
	var videoDurationSeconds sql.NullInt64
	var width sql.NullInt64
	var height sql.NullInt64
	var status string
	var visibility string
	var purpose string
	var uploadedByAuthID string
	var createdOn time.Time
	var updatedOn time.Time

	if err := scanner.Scan(
		&id,
		&key,
		&bucket,
		&originalName,
		&mimeType,
		&sizeBytes,
		&durationSeconds,
		&codec,
		&videoCodec,
		&audioCodec,
		&videoDurationSeconds,
		&width,
		&height,
		&status,
		&visibility,
		&purpose,
		&uploadedByAuthID,
		&createdOn,
		&updatedOn,
	); err != nil {
		return nil, fmt.Errorf("scanning file: %w", err)
	}

	baseMetadata, err := filedomain.NewFileMetadata(originalName, mimeType, sizeBytes)
	if err != nil {
		return nil, fmt.Errorf("restoring file metadata: %w", err)
	}
	var metadata filedomain.Metadata = baseMetadata
	hasAudioMetadata := durationSeconds.Valid || codec.Valid
	hasVideoMetadata := videoCodec.Valid || audioCodec.Valid || videoDurationSeconds.Valid || width.Valid || height.Valid
	if hasAudioMetadata && hasVideoMetadata {
		return nil, fmt.Errorf("restoring file metadata: %w", filedomain.ErrFileNotAvailable)
	}
	if hasAudioMetadata {
		if !durationSeconds.Valid || !codec.Valid || purpose != filedomain.PurposeConversationMessageAudio || status != filedomain.StatusConfirmed {
			return nil, fmt.Errorf("restoring file audio metadata: %w", filedomain.ErrMessageAudioNotAvailable)
		}
		metadata, err = filedomain.NewAudioFileMetadata(originalName, mimeType, sizeBytes, int(durationSeconds.Int64), codec.String)
		if err != nil {
			return nil, fmt.Errorf("restoring audio file metadata: %w", err)
		}
	} else if purpose == filedomain.PurposeConversationMessageAudio && status == filedomain.StatusConfirmed {
		return nil, fmt.Errorf("restoring confirmed audio file: %w", filedomain.ErrMessageAudioNotAvailable)
	}
	if hasVideoMetadata {
		if !videoCodec.Valid || !videoDurationSeconds.Valid || !width.Valid || !height.Valid || purpose != filedomain.PurposeConversationMessageVideo || status != filedomain.StatusConfirmed {
			return nil, fmt.Errorf("restoring file video metadata: %w", filedomain.ErrMessageVideoNotAvailable)
		}
		metadata, err = filedomain.NewVideoFileMetadata(originalName, mimeType, sizeBytes, filedomain.VideoMetadata{
			DurationSeconds: int(videoDurationSeconds.Int64),
			VideoCodec:      videoCodec.String,
			AudioCodec:      audioCodec.String,
			Width:           int(width.Int64),
			Height:          int(height.Int64),
		})
		if err != nil {
			return nil, fmt.Errorf("restoring video file metadata: %w", err)
		}
	} else if purpose == filedomain.PurposeConversationMessageVideo && status == filedomain.StatusConfirmed {
		return nil, fmt.Errorf("restoring confirmed video file: %w", filedomain.ErrMessageVideoNotAvailable)
	}
	file, err := filedomain.NewFile(id, key, bucket, metadata, status, visibility, purpose, uploadedByAuthID, createdOn, updatedOn)
	if err != nil {
		return nil, fmt.Errorf("restoring file: %w", err)
	}

	return file, nil
}

func rollbackFileTx(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback file transaction: %v", cause, rollbackErr)
	}

	return cause
}

func (repository *FileRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM files`)
	return err
}
