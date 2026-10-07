package notificationadapter

import (
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCompositeNotificatorAttemptsEveryChannelAfterFailure(t *testing.T) {
	failure := errors.New("channel failed")
	first, second := new(notificatorMock), new(notificatorMock)
	notice := &notification.Notification{ID: 1}
	first.On("Notify", mock.Anything, notice).Return(failure).Once()
	second.On("Notify", mock.Anything, notice).Return(nil).Once()
	require.ErrorIs(t, NewCompositeNotificator(first, nil, second).Notify(t.Context(), notice), failure)
	first.AssertExpectations(t)
	second.AssertExpectations(t)
}
func TestCompositeMessagePublisherInvokesBothExistingChannels(t *testing.T) {
	first, second := new(messagePublisherMock), new(messagePublisherMock)
	conv, err := conversation.NewPendingConversation(10, 20)
	require.NoError(t, err)
	message := conversation.Message{ID: 1}
	first.On("PublishMessage", mock.Anything, conv, "actor", message).Once()
	second.On("PublishMessage", mock.Anything, conv, "actor", message).Once()
	NewCompositeMessagePublisher(first, nil, second).PublishMessage(t.Context(), conv, "actor", message)
	first.AssertExpectations(t)
	second.AssertExpectations(t)
}
