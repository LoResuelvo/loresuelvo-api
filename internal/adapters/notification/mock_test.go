package notificationadapter

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/stretchr/testify/mock"
)

type notificatorMock struct{ mock.Mock }

func (m *notificatorMock) Notify(ctx context.Context, n *notification.Notification) error {
	return m.Called(ctx, n).Error(0)
}

type messagePublisherMock struct{ mock.Mock }

func (m *messagePublisherMock) PublishMessage(ctx context.Context, c conversation.Conversation, sender string, message conversation.Message) {
	m.Called(ctx, c, sender, message)
}
