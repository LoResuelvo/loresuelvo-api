package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/jackc/pgx/v5/pgconn"
)

type InstallationRepository struct{ db *sql.DB }

func NewInstallationRepository(db *sql.DB) *InstallationRepository {
	return &InstallationRepository{db: db}
}
func (r *InstallationRepository) Save(ctx context.Context, i *installation.Installation) error {
	hash := i.SecretHash
	if hash == nil {
		hash = []byte{}
	}
	var revision int64
	var err error
	if i.Revision == 0 {
		err = r.db.QueryRowContext(ctx, `INSERT INTO installations (id,user_id,app,token,locale,binding_id,enabled,secret_hash,revision,revoked) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1,$9) ON CONFLICT DO NOTHING RETURNING revision`, i.ID, i.UserID, i.App, i.Token, i.Locale, i.BindingID, i.Enabled, hash, i.Revoked).Scan(&revision)
	} else {
		err = r.db.QueryRowContext(ctx, `UPDATE installations SET user_id=$2,app=$3,token=$4,locale=$5,binding_id=$6,enabled=$7,secret_hash=$8,revoked=$9,revision=revision+1 WHERE id=$1 AND revision=$10 RETURNING revision`, i.ID, i.UserID, i.App, i.Token, i.Locale, i.BindingID, i.Enabled, hash, i.Revoked, i.Revision).Scan(&revision)
	}

	var postgresErr *pgconn.PgError
	if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &postgresErr) && postgresErr.Code == "23505") {
		return installation.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("saving installation: %w", err)
	}
	i.SecretHash = hash
	i.Revision = revision
	return nil
}
func (r *InstallationRepository) FindByID(ctx context.Context, id string) (*installation.Installation, error) {
	var i installation.Installation
	err := r.db.QueryRowContext(ctx, `SELECT id,user_id,app,token,locale,binding_id,enabled,secret_hash,revision,revoked FROM installations WHERE id=$1`, id).Scan(&i.ID, &i.UserID, &i.App, &i.Token, &i.Locale, &i.BindingID, &i.Enabled, &i.SecretHash, &i.Revision, &i.Revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, installation.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("finding installation: %w", err)
	}
	return &i, nil
}

func (r *InstallationRepository) FindByUserID(ctx context.Context, id int) ([]installation.Installation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id,app,token,locale,binding_id,enabled,secret_hash,revision,revoked FROM installations WHERE user_id=$1 AND enabled ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("finding installations: %w", err)
	}
	defer rows.Close()
	found := []installation.Installation{}
	for rows.Next() {
		var i installation.Installation
		if err := rows.Scan(&i.ID, &i.UserID, &i.App, &i.Token, &i.Locale, &i.BindingID, &i.Enabled, &i.SecretHash, &i.Revision, &i.Revoked); err != nil {
			return nil, fmt.Errorf("scanning installation: %w", err)
		}
		found = append(found, i)
	}
	return found, rows.Err()
}
