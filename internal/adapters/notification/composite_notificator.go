package notificationadapter

import (
	"context"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
)

type CompositeNotificator struct {
	channels []notification.Notificator
}

func NewCompositeNotificator(channels ...notification.Notificator) *CompositeNotificator {
	return &CompositeNotificator{channels: channels}
}

func (n *CompositeNotificator) Notify(ctx context.Context, notification *notification.Notification) error {
	for _, channel := range n.channels {
		if channel == nil {
			continue
		}
		if err := channel.Notify(ctx, notification); err != nil {
			return fmt.Errorf("notifying through channel: %w", err)
		}
	}
	return nil
}
