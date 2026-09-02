package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresConfigFromEnvParsesPoolSettings(t *testing.T) {
	t.Setenv(dbMaxOpenConnsEnv, "17")
	t.Setenv(dbMaxIdleConnsEnv, "6")
	t.Setenv(dbConnMaxLifetimeEnv, "45m")
	t.Setenv(dbConnMaxIdleTimeEnv, "7m")

	config, err := NewPostgresConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, 17, config.MaxOpenConns)
	assert.Equal(t, 6, config.MaxIdleConns)
	assert.Equal(t, 45*time.Minute, config.ConnMaxLifetime)
	assert.Equal(t, 7*time.Minute, config.ConnMaxIdleTime)
}

func TestNewPostgresConfigFromEnvUsesBoundedPoolDefaults(t *testing.T) {
	t.Setenv(dbMaxOpenConnsEnv, "")
	t.Setenv(dbMaxIdleConnsEnv, "")
	t.Setenv(dbConnMaxLifetimeEnv, "")
	t.Setenv(dbConnMaxIdleTimeEnv, "")

	config, err := NewPostgresConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, defaultMaxOpenConns, config.MaxOpenConns)
	assert.Equal(t, defaultMaxIdleConns, config.MaxIdleConns)
	assert.Equal(t, defaultConnMaxLifetime, config.ConnMaxLifetime)
	assert.Equal(t, defaultConnMaxIdleTime, config.ConnMaxIdleTime)
}

func TestNewPostgresConfigFromEnvRejectsInvalidPoolSettings(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected string
	}{
		{
			name:     "non integer max open connections",
			env:      map[string]string{dbMaxOpenConnsEnv: "many"},
			expected: dbMaxOpenConnsEnv,
		},
		{
			name:     "negative max idle connections",
			env:      map[string]string{dbMaxIdleConnsEnv: "-1"},
			expected: dbMaxIdleConnsEnv,
		},
		{
			name: "idle connections exceed open connections",
			env: map[string]string{
				dbMaxOpenConnsEnv: "2",
				dbMaxIdleConnsEnv: "3",
			},
			expected: dbMaxIdleConnsEnv,
		},
		{
			name:     "invalid connection lifetime",
			env:      map[string]string{dbConnMaxLifetimeEnv: "45"},
			expected: dbConnMaxLifetimeEnv,
		},
		{
			name:     "negative connection idle time",
			env:      map[string]string{dbConnMaxIdleTimeEnv: "-1m"},
			expected: dbConnMaxIdleTimeEnv,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, key := range []string{dbMaxOpenConnsEnv, dbMaxIdleConnsEnv, dbConnMaxLifetimeEnv, dbConnMaxIdleTimeEnv} {
				t.Setenv(key, test.env[key])
			}

			_, err := NewPostgresConfigFromEnv()

			require.Error(t, err)
			assert.ErrorContains(t, err, test.expected)
		})
	}
}

type poolConfigSpy struct {
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
}

func (spy *poolConfigSpy) SetMaxOpenConns(value int) {
	spy.maxOpenConns = value
}

func (spy *poolConfigSpy) SetMaxIdleConns(value int) {
	spy.maxIdleConns = value
}

func (spy *poolConfigSpy) SetConnMaxLifetime(value time.Duration) {
	spy.connMaxLifetime = value
}

func (spy *poolConfigSpy) SetConnMaxIdleTime(value time.Duration) {
	spy.connMaxIdleTime = value
}

func TestConfigurePoolAppliesAllLimits(t *testing.T) {
	spy := new(poolConfigSpy)
	config := PostgresConfig{
		MaxOpenConns:    17,
		MaxIdleConns:    6,
		ConnMaxLifetime: 45 * time.Minute,
		ConnMaxIdleTime: 7 * time.Minute,
	}

	configurePool(spy, config)

	assert.Equal(t, 17, spy.maxOpenConns)
	assert.Equal(t, 6, spy.maxIdleConns)
	assert.Equal(t, 45*time.Minute, spy.connMaxLifetime)
	assert.Equal(t, 7*time.Minute, spy.connMaxIdleTime)
}
