package testsupport

import (
	"context"
	"database/sql"
	"fmt"
)

// RemoveCurrentConsumerAddressForLegacyOperation creates the legacy state in
// which a persisted request still exists but its consumer's current address
// does not. Call only after the request has been created through the API.
func RemoveCurrentConsumerAddressForLegacyOperation(ctx context.Context, db *sql.DB, consumerID int) error {
	result, err := db.ExecContext(ctx, `DELETE FROM consumer_addresses WHERE consumer_id = $1`, consumerID)
	if err != nil {
		return fmt.Errorf("removing legacy consumer address: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking removed legacy consumer address: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("expected one current consumer address for %d, removed %d", consumerID, count)
	}
	return nil
}
