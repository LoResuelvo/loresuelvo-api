package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type OperationMediaReader struct{ db *sql.DB }

// OperationMediaFileFinder translates persistence absence into the media port's nil result.
type OperationMediaFileFinder struct{ files *FileRepository }

func NewOperationMediaFileFinder(files *FileRepository) *OperationMediaFileFinder {
	return &OperationMediaFileFinder{files: files}
}

func (finder *OperationMediaFileFinder) FindByID(ctx context.Context, id string) (*filedomain.File, error) {
	file, err := finder.files.FindByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return file, err
}

func NewOperationMediaReader(db *sql.DB) *OperationMediaReader {
	return &OperationMediaReader{db: db}
}

// The first proposal belongs to jr-N; later proposals are addressed by sp-N.
// Request images are shared with those later operations, but completion evidence
// is restricted to the work order belonging to the selected proposal.
const operationImageAssociationSQL = `WITH selected AS (
 SELECT jr.id AS job_request_id,
   (SELECT sp.id FROM service_proposals sp WHERE sp.conversation_id = jr.conversation_id ORDER BY sp.id LIMIT 1) AS proposal_id
 FROM job_requests jr WHERE $1::text = 'jr' AND jr.id = $2
 UNION ALL
 SELECT jr.id AS job_request_id, sp.id AS proposal_id
 FROM service_proposals sp
 LEFT JOIN job_requests jr ON jr.conversation_id = sp.conversation_id
 WHERE $1::text = 'sp' AND sp.id = $2
   AND (jr.id IS NULL OR sp.id <> (SELECT first.id FROM service_proposals first WHERE first.conversation_id = sp.conversation_id ORDER BY first.id LIMIT 1))
)
SELECT CASE
 WHEN EXISTS (SELECT 1 FROM job_request_images image WHERE image.job_request_id = selected.job_request_id AND image.file_id = $3::uuid)
   THEN 'job_request_image'
 WHEN EXISTS (
   SELECT 1 FROM work_orders wo
   JOIN work_order_completion_reports report ON report.work_order_id = wo.id
   JOIN work_order_completion_images image ON image.completion_report_id = report.id
   WHERE wo.service_proposal_id = selected.proposal_id AND image.file_id = $3::uuid
 ) THEN 'work_order_completion_image'
 END AS purpose
FROM selected`

func (reader *OperationMediaReader) FindAssociatedImagePurpose(ctx context.Context, id readmodel.ID, fileID string) (string, error) {
	var purpose sql.NullString
	err := reader.db.QueryRowContext(ctx, operationImageAssociationSQL, string(id.Kind), id.ResourceID, fileID).Scan(&purpose)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying operation image association: %w", err)
	}
	return purpose.String, nil
}
