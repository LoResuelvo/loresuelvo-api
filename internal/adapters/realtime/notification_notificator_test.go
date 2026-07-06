package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notificationAuthIDFinderStub struct {
	authID string
	role   string
	err    error
}

func (s notificationAuthIDFinderStub) FindByID(_ context.Context, id int) (user.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	base := &user.BaseUser{ID: id, AuthID: s.authID, Role: s.role}
	if s.role == provider.Role {
		return &provider.Provider{BaseUser: base}, nil
	}
	return &consumer.Consumer{BaseUser: base}, nil
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
	notificator := NewNotificationNotificator(hub, notificationAuthIDFinderStub{authID: authID, role: consumer.Role})
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

func TestNotificationNotificatorSendsNotificationToConnectedProvider(t *testing.T) {
	hub := NewHub()
	authID := "auth0|provider"
	connection := &Connection{
		hub:       hub,
		send:      make(chan []byte, 1),
		authID:    authID,
		role:      provider.Role,
		profileID: 20,
	}
	hub.addConnection(connection)
	notificator := NewNotificationNotificator(hub, notificationAuthIDFinderStub{
		authID: authID,
		role:   provider.Role,
	})
	notificationToSend := &notification.Notification{
		ID:           6,
		UserID:       20,
		Type:         notification.TypeServiceProposalAccepted,
		ResourceType: notification.ResourceServiceProposal,
		ResourceID:   100,
		CreatedAt:    time.Now().UTC(),
	}

	err := notificator.Notify(context.Background(), notificationToSend)

	require.NoError(t, err)
	payload := <-connection.send
	var event realtimeNotificationEvent
	require.NoError(t, json.Unmarshal(payload, &event))
	assert.Equal(t, notificationToSend.ID, event.Notification.ID)
	assert.Equal(t, notificationToSend.UserID, event.Notification.UserID)
	assert.Equal(t, string(notification.TypeServiceProposalAccepted), event.Notification.Type)
}

func TestNotificationNotificatorIgnoresDisconnectedConsumer(t *testing.T) {
	hub := NewHub()
	notificator := NewNotificationNotificator(hub, notificationAuthIDFinderStub{
		authID: "auth0|consumer",
		role:   consumer.Role,
	})

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
