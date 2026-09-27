package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OperationDetailMediaFixture backdates a confirmed upload to represent
// historical evidence while keeping the real upload and object-storage path.
type OperationDetailMediaFixture struct{ DB *sql.DB }

func (fixture OperationDetailMediaFixture) SetFileCreatedOn(ctx context.Context, fileID string, createdOn time.Time) error {
	result, err := fixture.DB.ExecContext(ctx, `UPDATE files SET created_on=$2 WHERE id=$1`, fileID, createdOn)
	if err != nil {
		return fmt.Errorf("setting media fixture creation time: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("media fixture file %s does not exist", fileID)
	}
	return nil
}
