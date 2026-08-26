package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

type UserRepository struct {
	db                     *sql.DB
	coverageZoneRepository *CoverageZoneRepository
}

const userWithProfileSelectSQL = `SELECT u.id, u.auth_id, u.email, u.name, u.surname, u.role,
	c.user_id, p.user_id, cat.id, cat.name, cat.normalized_name,
	COALESCE(u.profile_photo_file_id::text, ''), COALESCE(profile_photo.original_name, ''),
	consumer_address.street, consumer_address.street_number, consumer_address.floor,
	consumer_address.unit, consumer_address.latitude, consumer_address.longitude,
	consumer_address.coverage_zone_id,
	consumer_coverage_zone.id, consumer_coverage_zone.market_id, consumer_coverage_zone.code,
	consumer_coverage_zone.name, consumer_coverage_zone.normalized_name, consumer_coverage_zone.kind,
	consumer_coverage_zone.parent_zone_id, consumer_coverage_zone.enabled
	FROM users u
	LEFT JOIN consumers c ON c.user_id = u.id
	LEFT JOIN providers p ON p.user_id = u.id
	LEFT JOIN categories cat ON cat.id = p.category_id
	LEFT JOIN files profile_photo ON profile_photo.id = u.profile_photo_file_id
	LEFT JOIN consumer_addresses consumer_address ON consumer_address.consumer_id = c.user_id
	LEFT JOIN coverage_zones consumer_coverage_zone ON consumer_coverage_zone.id = consumer_address.coverage_zone_id`

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db:                     db,
		coverageZoneRepository: NewCoverageZoneRepository(db),
	}
}

func (repository *UserRepository) Save(ctx context.Context, userToSave user.User) (user.User, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning user transaction: %w", err)
	}

	userID, err := saveBaseUser(ctx, tx, userToSave)
	if err != nil {
		return nil, rollbackUserTx(tx, err)
	}
	userToSave.SetPersistenceID(userID)

	switch typedUser := userToSave.(type) {
	case *consumer.Consumer:
		if typedUser.Role() != consumer.Role {
			err = fmt.Errorf("consumer has role %q", typedUser.Role())
			break
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO consumers (user_id) VALUES ($1)`, typedUser.ID())
		if err == nil {
			err = saveConsumerAddress(ctx, tx, typedUser)
		}
	case *provider.Provider:
		if typedUser.Role() != provider.Role || typedUser.Category == nil {
			err = fmt.Errorf("provider has invalid role or category")
			break
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO providers (user_id, category_id) VALUES ($1, $2)`,
			typedUser.ID(), typedUser.Category.ID)
		if err == nil {
			err = saveProviderCoverageZones(ctx, tx, typedUser)
		}
	default:
		err = fmt.Errorf("saving user: unsupported user type %T", userToSave)
	}
	if err != nil {
		return nil, rollbackUserTx(tx, fmt.Errorf("saving user profile: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing user transaction: %w", err)
	}
	return userToSave, nil
}

func saveConsumerAddress(ctx context.Context, tx *sql.Tx, consumerToSave *consumer.Consumer) error {
	address := consumerToSave.Address()
	location := consumerToSave.Location()
	coverageZone := consumerToSave.CoverageZone()
	if coverageZone.ID <= 0 {
		return consumer.ErrConsumerAddressNotPersisted
	}

	if err := address.Validate(); err != nil {
		return err
	}
	if err := location.Validate(); err != nil {
		return err
	}

	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO consumer_addresses (
			consumer_id, street, street_number, floor, unit, latitude, longitude, coverage_zone_id
		) VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8)`,
		consumerToSave.ID(),
		address.Street,
		address.StreetNumber,
		address.Floor,
		address.Unit,
		location.Latitude,
		location.Longitude,
		coverageZone.ID,
	)
	if err != nil {
		return fmt.Errorf("saving consumer address: %w", err)
	}

	return nil
}

func saveProviderCoverageZones(ctx context.Context, tx *sql.Tx, providerToSave *provider.Provider) error {
	for _, zone := range providerToSave.CoverageZones {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO provider_coverage_zones (provider_id, coverage_zone_id) VALUES ($1, $2)`,
			providerToSave.ID(),
			zone.ID,
		); err != nil {
			return fmt.Errorf("saving provider coverage zone %d: %w", zone.ID, err)
		}
	}

	return nil
}

type userQueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func saveBaseUser(ctx context.Context, queryRower userQueryRower, userToSave user.User) (int, error) {
	var userID int
	err := queryRower.QueryRowContext(
		ctx,
		`INSERT INTO users (auth_id, email, name, surname, role, profile_photo_file_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id`,
		userToSave.AuthID(),
		userToSave.Email(),
		userToSave.Name(),
		userToSave.Surname(),
		userToSave.Role(),
		nullableImageFileID(userToSave.ProfilePhoto()),
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("saving user: %w", err)
	}

	return userID, nil
}

func (repository *UserRepository) FindByEmail(email string) bool {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`,
		email,
	).Scan(&exists)

	if err != nil {
		return false
	}

	return exists
}

func (repository *UserRepository) FindByAuthID(authID string) (user.User, error) {
	foundUser, err := scanUserWithProfile(
		repository.db.QueryRow(userWithProfileSelectSQL+" WHERE u.auth_id = $1", authID),
		"auth id",
	)
	if err != nil {
		return nil, err
	}

	return repository.hydrateUserCoverageZones(context.Background(), foundUser)
}

func (repository *UserRepository) FindConsumerByAuthID(ctx context.Context, authID string) (*consumer.Consumer, error) {
	foundUser, err := scanUserWithProfile(
		repository.db.QueryRowContext(ctx, userWithProfileSelectSQL+" WHERE u.auth_id = $1 AND u.role = $2", authID, consumer.Role),
		"consumer auth id",
	)
	if err != nil {
		return nil, err
	}

	foundConsumer, ok := foundUser.(*consumer.Consumer)
	if !ok {
		return nil, fmt.Errorf("finding consumer by auth id: user is not a consumer")
	}

	return foundConsumer, nil
}

func (repository *UserRepository) FindByID(ctx context.Context, id int) (user.User, error) {
	foundUser, err := scanUserWithProfile(
		repository.db.QueryRowContext(ctx, userWithProfileSelectSQL+" WHERE u.id = $1", id),
		"id",
	)
	if err != nil {
		return nil, err
	}

	return repository.hydrateUserCoverageZones(ctx, foundUser)
}

func (repository *UserRepository) hydrateUserCoverageZones(ctx context.Context, foundUser user.User) (user.User, error) {
	foundProvider, ok := foundUser.(*provider.Provider)
	if !ok {
		return foundUser, nil
	}

	zones, err := repository.coverageZoneRepository.FindByProviderID(ctx, foundProvider.ID())
	if err != nil {
		return nil, fmt.Errorf("hydrating provider coverage zones: %w", err)
	}
	foundProvider.CoverageZones = zones
	return foundProvider, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserWithProfile(row rowScanner, lookup string) (user.User, error) {
	var id int
	var authID, email, name, surname, role string
	var consumerID, providerID, categoryID sql.NullInt64
	var categoryName, normalizedCategoryName sql.NullString
	var profilePhotoFileID, profilePhotoOriginalName string
	var addressStreet, addressStreetNumber, addressFloor, addressUnit sql.NullString
	var addressLatitude, addressLongitude sql.NullFloat64
	var addressCoverageZoneID, coverageZoneID, coverageZoneMarketID, coverageZoneParentID sql.NullInt64
	var coverageZoneCode, coverageZoneName, coverageZoneNormalizedName, coverageZoneKind sql.NullString
	var coverageZoneEnabled sql.NullBool
	err := row.Scan(
		&id, &authID, &email, &name, &surname, &role,
		&consumerID, &providerID, &categoryID, &categoryName, &normalizedCategoryName,
		&profilePhotoFileID, &profilePhotoOriginalName,
		&addressStreet, &addressStreetNumber, &addressFloor, &addressUnit,
		&addressLatitude, &addressLongitude, &addressCoverageZoneID,
		&coverageZoneID, &coverageZoneMarketID, &coverageZoneCode, &coverageZoneName,
		&coverageZoneNormalizedName, &coverageZoneKind, &coverageZoneParentID, &coverageZoneEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("finding user by %s: %w", lookup, err)
	}
	base := user.RehydrateBaseUser(id, authID, email, name, surname, role, imageFromPersistence(profilePhotoFileID, profilePhotoOriginalName))
	switch role {
	case consumer.Role:
		if !consumerID.Valid {
			return nil, fmt.Errorf("finding user by %s: consumer profile is missing", lookup)
		}
		address, location, coverageZone, err := rehydrateConsumerAddress(
			addressStreet,
			addressStreetNumber,
			addressFloor,
			addressUnit,
			addressLatitude,
			addressLongitude,
			addressCoverageZoneID,
			coverageZoneID,
			coverageZoneMarketID,
			coverageZoneParentID,
			coverageZoneCode,
			coverageZoneName,
			coverageZoneNormalizedName,
			coverageZoneKind,
			coverageZoneEnabled,
		)
		if err != nil {
			return nil, fmt.Errorf("finding user by %s: %w", lookup, err)
		}
		return consumer.RehydrateConsumer(base, address, location, coverageZone), nil
	case provider.Role:
		if !providerID.Valid || !categoryID.Valid {
			return nil, fmt.Errorf("finding user by %s: provider profile is incomplete", lookup)
		}
		return &provider.Provider{
			BaseUser: base,
			Category: &category.Category{
				ID:             int(categoryID.Int64),
				Name:           categoryName.String,
				NormalizedName: normalizedCategoryName.String,
			},
		}, nil
	default:
		return nil, fmt.Errorf("finding user by %s: unsupported role %q", lookup, role)
	}
}

func rehydrateConsumerAddress(
	street, streetNumber, floor, unit sql.NullString,
	latitude, longitude sql.NullFloat64,
	addressCoverageZoneID, coverageZoneID, coverageZoneMarketID, coverageZoneParentID sql.NullInt64,
	coverageZoneCode, coverageZoneName, coverageZoneNormalizedName, coverageZoneKind sql.NullString,
	coverageZoneEnabled sql.NullBool,
) (consumer.Address, consumer.GeoPoint, coveragezone.CoverageZone, error) {
	if !street.Valid || !streetNumber.Valid || !latitude.Valid || !longitude.Valid ||
		!addressCoverageZoneID.Valid || !coverageZoneID.Valid || !coverageZoneMarketID.Valid ||
		!coverageZoneCode.Valid || !coverageZoneName.Valid || !coverageZoneNormalizedName.Valid ||
		!coverageZoneKind.Valid || !coverageZoneEnabled.Valid ||
		addressCoverageZoneID.Int64 != coverageZoneID.Int64 {
		return consumer.Address{}, consumer.GeoPoint{}, coveragezone.CoverageZone{}, consumer.ErrConsumerAddressNotPersisted
	}

	address, err := consumer.NewAddress(street.String, streetNumber.String, floor.String, unit.String)
	if err != nil {
		return consumer.Address{}, consumer.GeoPoint{}, coveragezone.CoverageZone{}, err
	}
	location, err := consumer.NewGeoPoint(latitude.Float64, longitude.Float64)
	if err != nil {
		return consumer.Address{}, consumer.GeoPoint{}, coveragezone.CoverageZone{}, err
	}

	var parentZoneID *int
	if coverageZoneParentID.Valid {
		parentZoneIDValue := int(coverageZoneParentID.Int64)
		parentZoneID = &parentZoneIDValue
	}
	coverageZone := coveragezone.CoverageZone{
		ID:             int(coverageZoneID.Int64),
		MarketID:       int(coverageZoneMarketID.Int64),
		Code:           coverageZoneCode.String,
		Name:           coverageZoneName.String,
		NormalizedName: coverageZoneNormalizedName.String,
		Kind:           coveragezone.Kind(coverageZoneKind.String),
		ParentZoneID:   parentZoneID,
		Enabled:        coverageZoneEnabled.Bool,
	}

	return *address, location, coverageZone, nil
}

func (repository *UserRepository) FindIDByAuthID(authID string) (int, error) {
	foundUser, err := repository.FindByAuthID(authID)
	if err != nil {
		return 0, err
	}
	return foundUser.ID(), nil
}

func rollbackUserTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback user transaction: %v", originalErr, rollbackErr)
	}
	return originalErr
}

func (repository *UserRepository) FindAuthIDByID(id int) (string, error) {
	var authID string
	if err := repository.db.QueryRow(`SELECT auth_id FROM users WHERE id = $1`, id).Scan(&authID); err != nil {
		return "", fmt.Errorf("finding user auth id by id: %w", err)
	}
	return authID, nil
}

func (repository *UserRepository) FindIDByEmail(email string) (int, error) {
	var id int
	if err := repository.db.QueryRow(`SELECT id FROM users WHERE email = $1`, email).Scan(&id); err != nil {
		return 0, fmt.Errorf("finding user id by email: %w", err)
	}
	return id, nil
}

func (repository *UserRepository) FindConsumerByID(consumerID int) (*consumer.Consumer, error) {
	foundUser, err := scanUserWithProfile(
		repository.db.QueryRow(userWithProfileSelectSQL+" WHERE u.id = $1", consumerID),
		"consumer id",
	)
	if err != nil {
		return nil, fmt.Errorf("finding consumer by id: %w", err)
	}

	foundConsumer, ok := foundUser.(*consumer.Consumer)
	if !ok {
		return nil, fmt.Errorf("finding consumer by id: user is not a consumer")
	}

	return foundConsumer, nil
}

// UpdateConsumerCoverageZone changes the zone associated with a persisted
// consumer address. The operation is intentionally constrained to enabled
// zones in enabled markets so an address cannot be moved into unavailable
// coverage.
func (repository *UserRepository) UpdateConsumerCoverageZone(ctx context.Context, consumerID, coverageZoneID int) error {
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE consumer_addresses AS address
		SET coverage_zone_id = zone.id, updated_on = NOW()
		FROM coverage_zones AS zone
		INNER JOIN coverage_markets AS market ON market.id = zone.market_id
		WHERE address.consumer_id = $1
			AND zone.id = $2
			AND zone.enabled = TRUE
			AND market.enabled = TRUE`,
		consumerID,
		coverageZoneID,
	)
	if err != nil {
		return fmt.Errorf("updating consumer coverage zone: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated consumer coverage zone: %w", err)
	}
	if rowsAffected == 0 {
		return consumer.ErrCoverageZoneNotAvailable
	}

	return nil
}

func (repository *UserRepository) ExistsProviderByID(id int) (bool, error) {
	var exists bool
	err := repository.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM providers WHERE user_id = $1)`,
		id,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking provider existence by id: %w", err)
	}

	return exists, nil
}

func (repository *UserRepository) FindProviderByID(ctx context.Context, providerID int) (*provider.Provider, error) {
	row := repository.db.QueryRowContext(ctx, providerSelectSQL+` WHERE providers.user_id = $1`, providerID)
	foundProvider, err := scanProvider(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, provider.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding provider by id: %w", err)
	}
	zones, err := repository.coverageZoneRepository.FindByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("hydrating provider coverage zones: %w", err)
	}
	foundProvider.CoverageZones = zones
	return foundProvider, nil
}

func (repository *UserRepository) FindProviderByAuthID(authID string) (*provider.Provider, error) {
	foundUser, err := repository.FindByAuthID(authID)
	if err != nil {
		return nil, fmt.Errorf("finding provider by auth id: %w", err)
	}
	foundProvider, ok := foundUser.(*provider.Provider)
	if !ok {
		return nil, provider.ErrDoesNotExist
	}
	return foundProvider, nil
}

func (repository *UserRepository) FindProvidersByCategoryID(categoryID int) ([]provider.Provider, error) {
	rows, err := repository.db.Query(
		providerSelectSQL+` WHERE providers.category_id = $1
		ORDER BY users.name ASC, users.surname ASC`,
		categoryID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	providers := []provider.Provider{}
	for rows.Next() {
		foundProvider, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, *foundProvider)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	providerIDs := make([]int, 0, len(providers))
	for index := range providers {
		providerIDs = append(providerIDs, providers[index].ID())
	}
	zonesByProviderID, err := repository.coverageZoneRepository.FindByProviderIDs(context.Background(), providerIDs)
	if err != nil {
		return nil, fmt.Errorf("hydrating provider coverage zones: %w", err)
	}
	for index := range providers {
		providers[index].CoverageZones = zonesByProviderID[providers[index].ID()]
	}

	return providers, nil
}

// FindProvidersByCategoryAndCoverageZoneID returns providers that belong to the
// requested category and explicitly cover the requested enabled zone.
func (repository *UserRepository) FindProvidersByCategoryAndCoverageZoneID(ctx context.Context, categoryID, coverageZoneID int) ([]provider.Provider, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		providerSelectSQL+`
	INNER JOIN provider_coverage_zones
		ON provider_coverage_zones.provider_id = providers.user_id
	INNER JOIN coverage_zones
		ON coverage_zones.id = provider_coverage_zones.coverage_zone_id
	INNER JOIN coverage_markets
		ON coverage_markets.id = coverage_zones.market_id
	WHERE providers.category_id = $1
		AND provider_coverage_zones.coverage_zone_id = $2
		AND coverage_zones.enabled = TRUE
		AND coverage_markets.enabled = TRUE
	ORDER BY users.name ASC, users.surname ASC`,
		categoryID,
		coverageZoneID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding providers by category and coverage zone: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	providers := make([]provider.Provider, 0)
	for rows.Next() {
		foundProvider, err := scanProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning provider by category and coverage zone: %w", err)
		}
		providers = append(providers, *foundProvider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating providers by category and coverage zone: %w", err)
	}

	providerIDs := make([]int, 0, len(providers))
	for index := range providers {
		providerIDs = append(providerIDs, providers[index].ID())
	}
	zonesByProviderID, err := repository.coverageZoneRepository.FindByProviderIDs(ctx, providerIDs)
	if err != nil {
		return nil, fmt.Errorf("hydrating provider coverage zones: %w", err)
	}
	for index := range providers {
		providers[index].CoverageZones = zonesByProviderID[providers[index].ID()]
	}

	return providers, nil
}

const providerSelectSQL = `SELECT providers.user_id,
	users.auth_id,
	users.email,
	users.name,
	users.surname,
	users.role,
	categories.id,
	categories.name,
	categories.normalized_name,
	COALESCE(users.profile_photo_file_id::text, ''),
	COALESCE(profile_photo.original_name, '')
FROM providers
INNER JOIN users ON users.id = providers.user_id
INNER JOIN categories ON categories.id = providers.category_id
LEFT JOIN files profile_photo ON profile_photo.id = users.profile_photo_file_id`

type providerScanner interface {
	Scan(dest ...any) error
}

func scanProvider(scanner providerScanner) (*provider.Provider, error) {
	var id int
	var authID, email, name, surname, role string
	var providerCategory category.Category
	var profilePhotoFileID, profilePhotoOriginalName string
	if err := scanner.Scan(
		&id,
		&authID,
		&email,
		&name,
		&surname,
		&role,
		&providerCategory.ID,
		&providerCategory.Name,
		&providerCategory.NormalizedName,
		&profilePhotoFileID,
		&profilePhotoOriginalName,
	); err != nil {
		return nil, err
	}
	return &provider.Provider{
		BaseUser: user.RehydrateBaseUser(id, authID, email, name, surname, role, imageFromPersistence(profilePhotoFileID, profilePhotoOriginalName)),
		Category: &providerCategory,
	}, nil
}

func imageFromPersistence(fileID, originalName string) *filedomain.Image {
	if fileID == "" {
		return nil
	}
	return &filedomain.Image{FileID: fileID, OriginalName: originalName}
}

func nullableImageFileID(image *filedomain.Image) sql.NullString {
	if image == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: image.FileID, Valid: true}
}

func (repository *UserRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM users`)
	return err
}

func (repository *UserRepository) DeleteAllOf(role string) error {
	_, err := repository.db.Exec(`DELETE FROM users WHERE role = $1`, role)
	return err
}
