package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
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
	notificator := NewNotificationNotificator(dispatcher, notificationRecipientFinderStub{authID: authID, role: consumer.Role}, nil, nil)
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
	for _, test := range []struct {
		name     string
		kind     notification.Type
		resource notification.ResourceType
	}{
		{"accepted proposal", notification.TypeServiceProposalAccepted, notification.ResourceServiceProposal},
		{"final payment", notification.TypeWorkOrderFinalPaymentApproved, notification.ResourceWorkOrder},
	} {
		t.Run(test.name, func(t *testing.T) {
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
			}, nil, nil)
			notificationToSend := &notification.Notification{
				ID:           6,
				UserID:       20,
				Type:         test.kind,
				ResourceType: test.resource,
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
			assert.Equal(t, "notification.created", event.Type)
			assert.Equal(t, string(test.kind), event.Notification.Type)
			assert.Equal(t, string(test.resource), event.Notification.ResourceType)
			assert.Equal(t, notificationToSend.ResourceID, event.Notification.ResourceID)

		})
	}
}

func TestNotificationNotificatorIgnoresDisconnectedConsumer(t *testing.T) {
	hub := NewHub()
	eventBus := new(eventBusMock)
	eventBus.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	dispatcher := NewDispatcher(hub, eventBus)
	notificator := NewNotificationNotificator(dispatcher, notificationRecipientFinderStub{
		authID: "auth0|consumer",
		role:   consumer.Role,
	}, nil, nil)

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

func TestJobRequestNotificationIncludesResolvedRequest(t *testing.T) {
	for _, attached := range []bool{false, true} {
		t.Run(map[bool]string{false: "without images", true: "with images"}[attached], func(t *testing.T) {
			requests := new(notificationJobRequestFinderMock)
			users := new(notificationUserFinderMock)
			images := new(notificationImageResolverMock)
			dispatcher := new(notificationDispatcherMock)
			request := &jobrequest.JobRequest{ID: 99, ConsumerID: 10, ProviderID: 20, ConversationID: 30, Title: "Leak", Description: "Repair", Status: jobrequest.StatusPending}
			resolved := []filedomain.Image{}
			if attached {
				request.Images = []filedomain.Image{{FileID: "image", OriginalName: "leak.jpg"}}
				resolved = []filedomain.Image{{FileID: "image", OriginalName: "leak.jpg", URL: "https://storage.example/resolved"}}
			}
			requests.On("FindByID", 99).Return(request, nil).Once()
			users.On("FindByID", mock.Anything, 20).Return(user.RehydrateBaseUser(20, "auth-provider", "", "Juan", "Perez", provider.Role, nil), nil).Once()
			users.On("FindByID", mock.Anything, 10).Return(user.RehydrateBaseUser(10, "auth-consumer", "", "Ana", "Gomez", consumer.Role, nil), nil).Once()
			images.On("ResolveJobRequestImages", mock.Anything, request.Images).Return(resolved, nil).Once()
			dispatcher.On("Publish", mock.Anything, "auth-provider", provider.Role, 20, mock.Anything).Run(func(args mock.Arguments) {
				var event realtimeNotificationEvent
				require.NoError(t, json.Unmarshal(args.Get(4).([]byte), &event))
				require.NotNil(t, event.JobRequest)
				assert.Equal(t, 99, event.Notification.ResourceID)
				assert.Equal(t, 99, event.JobRequest.ID)
				assert.Equal(t, 30, event.JobRequest.ConversationID)
				assert.Equal(t, "Leak", event.JobRequest.Title)
				assert.Equal(t, "Repair", event.JobRequest.Description)
				assert.Equal(t, "pending", event.JobRequest.Status)
				assert.Equal(t, realtimeJobRequester{Name: "Ana", Surname: "Gomez"}, event.JobRequest.Requester)
				require.NotNil(t, event.JobRequest.Images)
				if attached {
					assert.Equal(t, []realtimeMessageImage{{ID: "image", OriginalName: "leak.jpg", URL: "https://storage.example/resolved"}}, event.JobRequest.Images)
				} else {
					assert.Empty(t, event.JobRequest.Images)
				}
			}).Return(nil).Once()
			adapter := NewNotificationNotificator(dispatcher, users, requests, images)
			require.NoError(t, adapter.Notify(t.Context(), &notification.Notification{ID: 7, UserID: 20, Type: notification.TypeJobRequestReceived, ResourceType: notification.ResourceJobRequest, ResourceID: 99}))
			requests.AssertExpectations(t)
			users.AssertExpectations(t)
			images.AssertExpectations(t)
			dispatcher.AssertExpectations(t)
		})
	}
}

func TestJobRequestNotificationDoesNotPublishUnresolvedOrMisaddressedRequest(t *testing.T) {
	for _, phase := range []string{"request lookup", "recipient mismatch", "consumer lookup", "image resolution", "publish"} {
		t.Run(phase, func(t *testing.T) {
			failure := errors.New("unavailable")
			requests := new(notificationJobRequestFinderMock)
			users := new(notificationUserFinderMock)
			images := new(notificationImageResolverMock)
			dispatcher := new(notificationDispatcherMock)
			request := &jobrequest.JobRequest{ID: 99, ConsumerID: 10, ProviderID: 20}
			users.On("FindByID", mock.Anything, 20).Return(user.RehydrateBaseUser(20, "auth-provider", "", "Juan", "Perez", provider.Role, nil), nil).Once()
			switch phase {
			case "request lookup":
				requests.On("FindByID", 99).Return((*jobrequest.JobRequest)(nil), failure).Once()
			default:
				if phase == "recipient mismatch" {
					request.ProviderID = 21
				}
				requests.On("FindByID", 99).Return(request, nil).Once()
				if phase != "recipient mismatch" {
					consumerErr := error(nil)
					if phase == "consumer lookup" {
						consumerErr = failure
					}
					users.On("FindByID", mock.Anything, 10).Return(user.RehydrateBaseUser(10, "auth-consumer", "", "Ana", "Gomez", consumer.Role, nil), consumerErr).Once()
					if phase != "consumer lookup" {
						imageErr := error(nil)
						if phase == "image resolution" {
							imageErr = failure
						}
						images.On("ResolveJobRequestImages", mock.Anything, request.Images).Return([]filedomain.Image{}, imageErr).Once()
						if phase == "publish" {
							dispatcher.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(failure).Once()
						}
					}
				}
			}
			adapter := NewNotificationNotificator(dispatcher, users, requests, images)
			err := adapter.Notify(t.Context(), &notification.Notification{ID: 7, UserID: 20, Type: notification.TypeJobRequestReceived, ResourceType: notification.ResourceJobRequest, ResourceID: 99})
			if phase == "recipient mismatch" {
				require.ErrorContains(t, err, "not assigned")
			} else {
				require.ErrorIs(t, err, failure)
			}
			if phase != "publish" {
				dispatcher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			requests.AssertExpectations(t)
			users.AssertExpectations(t)
			images.AssertExpectations(t)
			dispatcher.AssertExpectations(t)
		})
	}
}
