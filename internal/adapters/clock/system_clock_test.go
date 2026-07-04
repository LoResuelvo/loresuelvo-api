package clock_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/stretchr/testify/assert"
)

func TestTrueNow(t *testing.T) {
	clock := clock.NewSystemClock()
	now := clock.Now()
	assert.WithinDuration(t, time.Now(), now, time.Second)
}

func TestSetTime(t *testing.T) {
	clock := clock.NewSystemClock()
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clock.SetTime(fixedTime)

	now := clock.Now()
	assert.Equal(t, fixedTime, now)
}

func TestReset(t *testing.T) {
	clock := clock.NewSystemClock()
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clock.SetTime(fixedTime)

	clock.Reset()
	now := clock.Now()
	assert.WithinDuration(t, time.Now(), now, time.Second)
}
