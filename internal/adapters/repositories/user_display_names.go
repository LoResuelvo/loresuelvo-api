package repositories

import (
	"context"
	"fmt"
)

// FindNamesByIDs returns current display names without changing historical records.
func (repository *UserRepository) FindNamesByIDs(ctx context.Context, ids []int) (map[int]string, error) {
	names := make(map[int]string)
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := repository.db.QueryContext(ctx, `SELECT id, trim(concat_ws(' ', NULLIF(trim(name),''), NULLIF(trim(surname),''))) FROM users WHERE id=ANY($1::integer[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("finding user display names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scanning user display name: %w", err)
		}
		if name != "" {
			names[id] = name
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading user display names: %w", err)
	}
	return names, nil
}
