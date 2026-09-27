package testsupport

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OperationDetailReportFixture replaces the generic completion evidence created
// by the shared work-order fixture with the historical evidence in the scenario.
type OperationDetailReportFixture struct{ DB *sql.DB }

func (fixture OperationDetailReportFixture) ReplaceCompletionReport(ctx context.Context, orderID int, description string, reportedOn time.Time, fileIDs []string) (err error) {
	tx, err := fixture.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning report fixture: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var reportID int
	if err = tx.QueryRowContext(ctx, `UPDATE work_order_completion_reports SET description = $2, reported_on = $3 WHERE work_order_id = $1 RETURNING id`, orderID, description, reportedOn).Scan(&reportID); err != nil {
		return fmt.Errorf("replacing completion report for order %d: %w", orderID, err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM work_order_completion_images WHERE completion_report_id = $1`, reportID); err != nil {
		return fmt.Errorf("clearing generic completion images: %w", err)
	}
	for position, fileID := range fileIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO work_order_completion_images (completion_report_id, file_id, position) VALUES ($1, $2, $3)`, reportID, fileID, position); err != nil {
			return fmt.Errorf("linking completion image %d: %w", position, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing report fixture: %w", err)
	}
	return nil
}
