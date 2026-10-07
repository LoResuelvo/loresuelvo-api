package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
)

type InstallationRepository struct{ db *sql.DB }

func NewInstallationRepository(db *sql.DB) *InstallationRepository {
	return &InstallationRepository{db: db}
}
func (r *InstallationRepository) Save(ctx context.Context, i *installation.Installation) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO installations (id,user_id,app,token,locale,binding_id,enabled) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (id) DO UPDATE SET user_id=EXCLUDED.user_id,app=EXCLUDED.app,token=EXCLUDED.token,locale=EXCLUDED.locale,binding_id=EXCLUDED.binding_id,enabled=EXCLUDED.enabled`, i.ID, i.UserID, i.App, i.Token, i.Locale, i.BindingID, i.Enabled)
	if err != nil {
		return fmt.Errorf("saving installation: %w", err)
	}
	return nil
}
func (r *InstallationRepository) FindByUserID(ctx context.Context, id int) ([]installation.Installation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,app,token,locale,binding_id,enabled FROM installations WHERE user_id=$1 AND enabled ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("finding installations: %w", err)
	}
	defer rows.Close()
	found := []installation.Installation{}
	for rows.Next() {
		var i installation.Installation
		if err := rows.Scan(&i.ID, &i.UserID, &i.App, &i.Token, &i.Locale, &i.BindingID, &i.Enabled); err != nil {
			return nil, fmt.Errorf("scanning installation: %w", err)
		}
		found = append(found, i)
	}
	return found, rows.Err()
}
