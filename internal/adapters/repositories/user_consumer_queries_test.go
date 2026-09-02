package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newConsumerRepositoryTest(t *testing.T) (*repositories.UserRepository, *sql.DB) {
	t.Helper()

	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM users")
		database.Close()
	})

	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	userRepository := repositories.NewUserRepository(database)
	return userRepository, database
}

func validConsumer(t *testing.T, database *sql.DB) consumer.Consumer {
	return *consumerWithAddress(t, database, "auth0|josue", "josugod@gmail.com", "Josue", "el pro")
}

func consumerUser(value consumer.Consumer) *consumer.Consumer {
	return &value
}

func TestConsumerRepositoryCanSaveAConsumer(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)
	consumer := validConsumer(t, database)

	_, err := repo.Save(context.Background(), consumerUser(consumer))

	assert.NoError(t, err)
	exists := repo.FindByEmail(consumer.Email())
	assert.True(t, exists, "Consumer should be saved on database")
}

func TestConsumerRepositoryCanDeleteAllConsumers(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)

	_, _ = repo.Save(context.Background(), consumerUser(validConsumer(t, database)))

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validConsumer(t, database).Email())
	assert.False(t, exists, "All consumers should be deleted from database")
}

func TestConsumerRepositoryCanFindByEmail(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)
	consumer := validConsumer(t, database)

	_, err := repo.Save(context.Background(), consumerUser(consumer))

	assert.NoError(t, err, "saving consumer should not return an error")
	assert.True(t, repo.FindByEmail(consumer.Email()), "Consumer should be found by email")
}

func TestConsumerRepositoryFindByEmailReturnsFalseIfConsumerDoesNotExist(t *testing.T) {
	repo, _ := newConsumerRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Consumer should not be found by email if it does not exist")
}

func TestConsumerRepositoryCanFindIDByEmail(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)
	consumer := validConsumer(t, database)

	_, err := repo.Save(context.Background(), consumerUser(consumer))
	require.NoError(t, err, "saving consumer should not return an error")

	consumerID, err := repo.FindIDByEmail(consumer.Email())

	require.NoError(t, err)
	assert.NotZero(t, consumerID)
}

func TestConsumerRepositoryCanFindIDByAuthID(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)
	consumer := validConsumer(t, database)

	_, err := repo.Save(context.Background(), consumerUser(consumer))
	require.NoError(t, err, "saving consumer should not return an error")

	consumerID, err := repo.FindIDByAuthID(consumer.AuthID())

	require.NoError(t, err)
	assert.NotZero(t, consumerID)
}

func TestConsumerRepositoryCanFindConsumerByAuthID(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)
	consumerToSave := validConsumer(t, database)

	_, err := repo.Save(context.Background(), consumerUser(consumerToSave))
	require.NoError(t, err, "saving consumer should not return an error")

	foundConsumer, err := repo.FindConsumerByAuthID(context.Background(), consumerToSave.AuthID())

	require.NoError(t, err)
	assert.Equal(t, consumerToSave.ID(), foundConsumer.ID())
	assert.Equal(t, consumerToSave.CoverageZone(), foundConsumer.CoverageZone())
}

func TestConsumerRepositoryCanFindByID(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)
	consumerToSave := validConsumer(t, database)

	_, err := repo.Save(context.Background(), consumerUser(consumerToSave))
	require.NoError(t, err, "saving consumer should not return an error")
	consumerID, err := repo.FindIDByEmail(consumerToSave.Email())
	require.NoError(t, err)

	foundConsumer, err := repo.FindConsumerByID(consumerID)

	require.NoError(t, err)
	assert.Equal(t, consumerID, foundConsumer.ID())
	require.NotNil(t, foundConsumer.BaseUser)
	assert.Equal(t, consumerToSave.AuthID(), foundConsumer.AuthID())
	assert.Equal(t, consumerToSave.Email(), foundConsumer.Email())
	assert.Equal(t, consumerToSave.Name(), foundConsumer.Name())
	assert.Equal(t, consumerToSave.Surname(), foundConsumer.Surname())
	assert.Equal(t, consumerToSave.Role(), foundConsumer.Role())
}

func TestConsumerRepositoryRejectsConsumerWithoutAddressOnRead(t *testing.T) {
	repo, database := newConsumerRepositoryTest(t)

	var userID int
	err := database.QueryRow(
		`INSERT INTO users (auth_id, email, name, surname, role, created_on, updated_on)
		VALUES ('auth0|consumer-without-address', 'consumer-without-address@example.com', 'Ana', 'Perez', 'consumer', NOW(), NOW())
		RETURNING id`,
	).Scan(&userID)
	require.NoError(t, err)

	_, err = database.Exec(`INSERT INTO consumers (user_id) VALUES ($1)`, userID)
	require.NoError(t, err)

	_, err = repo.FindConsumerByID(userID)

	assert.ErrorIs(t, err, consumer.ErrConsumerAddressNotPersisted)
}

func TestConsumerRepositoryPersistsAndRehydratesConsumerAddress(t *testing.T) {
	repository, coverageZone := newConsumerRepositoryTestWithCoverageZone(t)
	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "4", "B")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)
	consumerToSave, err := consumer.NewConsumer(
		"auth0|consumer-address",
		"consumer-address@example.com",
		"Ana",
		"Perez",
		nil,
		address,
		location,
		*coverageZone,
	)
	require.NoError(t, err)

	_, err = repository.Save(context.Background(), consumerToSave)
	require.NoError(t, err)

	foundConsumer, err := repository.FindConsumerByID(consumerToSave.ID())

	require.NoError(t, err)
	assert.Equal(t, *address, foundConsumer.Address())
	assert.Equal(t, location, foundConsumer.Location())
	assert.Equal(t, *coverageZone, foundConsumer.CoverageZone())
}

func TestConsumerRepositoryCanUpdateCoverageZone(t *testing.T) {
	repository, coverageZone := newConsumerRepositoryTestWithCoverageZone(t)
	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "", "")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)
	consumerToSave, err := consumer.NewConsumer(
		"auth0|consumer-address-update",
		"consumer-address-update@example.com",
		"Ana",
		"Perez",
		nil,
		address,
		location,
		*coverageZone,
	)
	require.NoError(t, err)

	_, err = repository.Save(context.Background(), consumerToSave)
	require.NoError(t, err)

	err = repository.UpdateConsumerCoverageZone(context.Background(), consumerToSave.ID(), coverageZone.ID)

	require.NoError(t, err)
	foundConsumer, err := repository.FindConsumerByID(consumerToSave.ID())
	require.NoError(t, err)
	assert.Equal(t, coverageZone.ID, foundConsumer.CoverageZone().ID)
}

func TestConsumerRepositoryRollsBackConsumerAddressFailure(t *testing.T) {
	repository, _ := newConsumerRepositoryTest(t)
	address, err := consumer.NewAddress("Av. Rivadavia", "5100", "", "")
	require.NoError(t, err)
	location, err := consumer.NewGeoPoint(-34.6208, -58.4386)
	require.NoError(t, err)
	consumerToSave, err := consumer.NewConsumer(
		"auth0|consumer-address-rollback",
		"consumer-address-rollback@example.com",
		"Ana",
		"Perez",
		nil,
		address,
		location,
		coveragezone.CoverageZone{ID: 999999999, Name: "Missing zone", Enabled: true},
	)
	require.NoError(t, err)

	_, err = repository.Save(context.Background(), consumerToSave)

	require.Error(t, err)
	assert.False(t, repository.FindByEmail(consumerToSave.Email()))
}

func newConsumerRepositoryTestWithCoverageZone(t *testing.T) (*repositories.UserRepository, *coveragezone.CoverageZone) {
	t.Helper()

	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)
	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	var marketID int
	err = database.QueryRow(
		`INSERT INTO coverage_markets (code, name, enabled, created_on, updated_on)
		VALUES ('CABA-CONSUMER-TEST', 'Consumer test market', TRUE, NOW(), NOW())
		RETURNING id`,
	).Scan(&marketID)
	require.NoError(t, err)

	var coverageZoneID int
	err = database.QueryRow(
		`INSERT INTO coverage_zones (market_id, code, name, normalized_name, kind, enabled, created_on, updated_on)
		VALUES ($1, 'CABA-CONSUMER-TEST-01', 'Consumer Test Zone', 'consumer test zone', 'COMMUNE', TRUE, NOW(), NOW())
		RETURNING id`,
		marketID,
	).Scan(&coverageZoneID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM users")
		_, _ = database.Exec("DELETE FROM coverage_zones WHERE id = $1", coverageZoneID)
		_, _ = database.Exec("DELETE FROM coverage_markets WHERE id = $1", marketID)
		_ = database.Close()
	})

	coverageZone, err := repositories.NewCoverageZoneRepository(database).FindByID(context.Background(), coverageZoneID)
	require.NoError(t, err, "could not find consumer test coverage zone")

	return repositories.NewUserRepository(database), coverageZone
}
