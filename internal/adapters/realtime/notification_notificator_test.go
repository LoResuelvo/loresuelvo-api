package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
	eventBus := new(eventBusMock)
	eventBus.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	dispatcher := NewDispatcher(hub, eventBus)
	notificator := NewNotificationNotificator(dispatcher, notificationRecipientFinderStub{authID: authID, role: consumer.Role})
	createdAt := time.Date(2026, 7, 4, 13, 0, 0, 0, time.UTC)
	notificationToSend := &notification.Notification{
		ID:                       5,
		UserID:                   10,
		Type:                     notification.TypeServiceProposalReceived,
		ResourceType:             notification.ResourceServiceProposal,
		ResourceID:               99,
		EstimatedDurationMinutes: 90,
		CreatedAt:                createdAt,
	}

	err := notificator.Notify(context.Background(), notificationToSend)

	require.NoError(t, err)
	eventBus.AssertExpectations(t)
	payload := <-connection.send
	var event realtimeNotificationEvent
	require.NoError(t, json.Unmarshal(payload, &event))
	assert.Equal(t, "notification.created", event.Type)
	assert.Equal(t, notificationToSend.ID, event.Notification.ID)
	assert.Equal(t, notificationToSend.UserID, event.Notification.UserID)
	assert.Equal(t, string(notificationToSend.Type), event.Notification.Type)
	assert.Equal(t, string(notificationToSend.ResourceType), event.Notification.ResourceType)
	assert.Equal(t, notificationToSend.ResourceID, event.Notification.ResourceID)
	assert.Equal(t, notificationToSend.EstimatedDurationMinutes, event.Notification.EstimatedDurationMinutes)
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
	eventBus := new(eventBusMock)
	eventBus.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	dispatcher := NewDispatcher(hub, eventBus)
	notificator := NewNotificationNotificator(dispatcher, notificationRecipientFinderStub{
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
	eventBus.AssertExpectations(t)
	payload := <-connection.send
	var event realtimeNotificationEvent
	require.NoError(t, json.Unmarshal(payload, &event))
	assert.Equal(t, notificationToSend.ID, event.Notification.ID)
	assert.Equal(t, notificationToSend.UserID, event.Notification.UserID)
	assert.Equal(t, string(notification.TypeServiceProposalAccepted), event.Notification.Type)
}

func TestNotificationNotificatorIgnoresDisconnectedConsumer(t *testing.T) {
	hub := NewHub()
	eventBus := new(eventBusMock)
	eventBus.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	dispatcher := NewDispatcher(hub, eventBus)
	notificator := NewNotificationNotificator(dispatcher, notificationRecipientFinderStub{
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
	eventBus.AssertExpectations(t)
}
