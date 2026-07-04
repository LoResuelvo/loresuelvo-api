package clock

import (
	"sync"
	"time"
)

type SystemClock struct {
	mu         sync.RWMutex
	mockedTime *time.Time
}

func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

func (c *SystemClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mockedTime != nil {
		return *c.mockedTime
	}

	return time.Now().UTC()
}

func (c *SystemClock) SetTime(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	utc := t.UTC()
	c.mockedTime = &utc
}

func (c *SystemClock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.mockedTime = nil
}
