package realtime

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type TicketStore struct {
	mu      sync.RWMutex
	tickets map[string]ticketInfo
}

type ticketInfo struct {
	authID    string
	expiresAt time.Time
}

func NewTicketStore() *TicketStore {
	return &TicketStore{
		tickets: make(map[string]ticketInfo),
	}
}

// Issue generates a new secure random ticket that expires in 1 minute
func (s *TicketStore) Issue(authID string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(bytes)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up old tickets lazily to avoid memory leaks
	now := time.Now()
	for k, v := range s.tickets {
		if now.After(v.expiresAt) {
			delete(s.tickets, k)
		}
	}

	s.tickets[ticket] = ticketInfo{
		authID:    authID,
		expiresAt: now.Add(1 * time.Minute),
	}

	return ticket, nil
}

// Consume validates a ticket and returns the associated authID.
// A ticket can only be consumed once.
func (s *TicketStore) Consume(ticket string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, ok := s.tickets[ticket]
	if !ok {
		return "", false
	}

	// Delete immediately so it's one-time use
	delete(s.tickets, ticket)

	if time.Now().After(info.expiresAt) {
		return "", false
	}

	return info.authID, true
}
