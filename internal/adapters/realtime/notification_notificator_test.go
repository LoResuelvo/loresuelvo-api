package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notificationAuthIDFinderStub struct {
	authID string
	err    error
}

func (s notificationAuthIDFinderStub) FindAuthIDByID(id int) (string, error) {
	return s.authID, s.err
}

func TestNotificationNotificatorSendsNotificationToConnectedConsumer(t *testing.T) {
	hub := NewHub()
	authID := "auth0|consumer"
	connection := &Connection{
		hub:       hub,
		send:      make(chan []byte, 1),
		authID:    authID,
		role:      consumer.Role,
		profileID: 10,
	}
	hub.addConnection(connection)
	notificator := NewNotificationNotificator(hub, notificationAuthIDFinderStub{authID: authID})
	createdAt := time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC)
	notificationToSend := &notification.Notification{
		ID:           5,
		UserID:       10,
		Type:         notification.TypeServiceProposalReceived,
		ResourceType: notification.ResourceServiceProposal,
		ResourceID:   99,
		CreatedAt:    createdAt,
	}

	err := notificator.Notify(context.Background(), notificationToSend)

	require.NoError(t, err)
	payload := <-connection.send
	var event realtimeNotificationEvent
	require.NoError(t, json.Unmarshal(payload, &event))
	assert.Equal(t, "notification.created", event.Type)
	assert.Equal(t, notificationToSend.ID, event.Notification.ID)
	assert.Equal(t, notificationToSend.UserID, event.Notification.UserID)
	assert.Equal(t, string(notificationToSend.Type), event.Notification.Type)
	assert.Equal(t, string(notificationToSend.ResourceType), event.Notification.ResourceType)
	assert.Equal(t, notificationToSend.ResourceID, event.Notification.ResourceID)
	assert.Nil(t, event.Notification.ReadAt)
	assert.Equal(t, createdAt, event.Notification.CreatedAt)
}

func TestNotificationNotificatorIgnoresDisconnectedConsumer(t *testing.T) {
	hub := NewHub()
	notificator := NewNotificationNotificator(hub, notificationAuthIDFinderStub{authID: "auth0|consumer"})

	err := notificator.Notify(context.Background(), &notification.Notification{
		ID:           5,
		UserID:       10,
		Type:         notification.TypeServiceProposalReceived,
		ResourceType: notification.ResourceServiceProposal,
		ResourceID:   99,
		CreatedAt:    time.Now().UTC(),
	})

	require.NoError(t, err)
}
