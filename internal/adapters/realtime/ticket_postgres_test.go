package realtime

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostgresTicketStoreCanBeConsumedOnlyOnceAcrossStores(t *testing.T) {
	databaseA := openRealtimeTestDatabase(t)
	databaseB := openRealtimeTestDatabase(t)
	storeA := NewPostgresTicketStore(databaseA)
	storeB := NewPostgresTicketStore(databaseB)
	ticket, err := storeA.Issue(context.Background(), "realtime-integration-consumer")
	require.NoError(t, err)

	const consumers = 8
	var waitGroup sync.WaitGroup
	type consumeResult struct {
		authID string
		valid  bool
		err    error
	}
	results := make(chan consumeResult, consumers)
	waitGroup.Add(consumers)
	for index := range consumers {
		store := storeA
		if index%2 == 1 {
			store = storeB
		}
		go func() {
			defer waitGroup.Done()
			authID, valid, consumeErr := store.Consume(context.Background(), ticket)
			results <- consumeResult{authID: authID, valid: valid, err: consumeErr}
		}()
	}
	waitGroup.Wait()
	close(results)

	var consumed []string
	for result := range results {
		require.NoError(t, result.err)
		if result.valid {
			consumed = append(consumed, result.authID)
		}
	}
	require.Equal(t, []string{"realtime-integration-consumer"}, consumed)
}

func TestPostgresTicketStoreRejectsExpiredTickets(t *testing.T) {
	database := openRealtimeTestDatabase(t)
	store := NewPostgresTicketStore(database)
	ticket, err := store.Issue(context.Background(), "realtime-integration-consumer")
	require.NoError(t, err)

	_, err = database.ExecContext(
		context.Background(),
		`UPDATE websocket_tickets SET expires_at = NOW() - INTERVAL '1 second' WHERE ticket = $1`,
		ticket,
	)
	require.NoError(t, err)

	authID, valid, err := store.Consume(context.Background(), ticket)
	require.NoError(t, err)
	require.False(t, valid)
	require.Empty(t, authID)
}
