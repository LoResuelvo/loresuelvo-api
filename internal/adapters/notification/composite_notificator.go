package notificationadapter

import (
	"context"
	"errors"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
)

type CompositeNotificator struct {
	channels []notification.Notificator
}

func NewCompositeNotificator(channels ...notification.Notificator) *CompositeNotificator {
	return &CompositeNotificator{channels: channels}
}

func (n *CompositeNotificator) Notify(ctx context.Context, notification *notification.Notification) error {
	var failures []error
	for _, channel := range n.channels {
		if channel == nil {
			continue
		}
		if err := channel.Notify(ctx, notification); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
