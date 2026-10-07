package push

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type installationStore interface {
	Save(context.Context, *installation.Installation) error
	FindByUserID(context.Context, int) ([]installation.Installation, error)
}
type orderFinder interface {
	FindByID(context.Context, int) (*workorder.WorkOrder, error)
	FindByServiceProposalID(context.Context, int) (*workorder.WorkOrder, error)
}

type Publisher struct {
	sender        *Sender
	installations installationStore
	orders        orderFinder
	clock         clock.Clock
}

func NewPublisher(sender *Sender, installations installationStore, orders orderFinder, clock clock.Clock) *Publisher {
	return &Publisher{sender: sender, installations: installations, orders: orders, clock: clock}
}
func (p *Publisher) Notify(ctx context.Context, n *notification.Notification) error {
	if p.sender == nil || n == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, p.sender.timeout())
	defer cancel()
	kind := string(n.Type)
	if _, ok := templates[kind]; !ok {
		return nil
	}
	resource, resourceID := string(n.ResourceType), n.ResourceID
	expires := n.CreatedAt.Add(24 * time.Hour)
	if n.Type == notification.TypeServiceProposalAccepted {
		order, err := p.orders.FindByServiceProposalID(ctx, n.ResourceID)
		if err != nil {
			p.diagnose(kind, "destination_lookup")
			return nil
		}
		resource, resourceID = "work_order", order.ID()
	}
	if n.Type == notification.TypeWorkOrderCloseToScheduledTime {
		order, err := p.orders.FindByID(ctx, n.ResourceID)
		if err != nil {
			p.diagnose(kind, "destination_lookup")
			return nil
		}
		expires = order.ScheduledOn()
	}
	p.deliver(ctx, n.UserID, fmt.Sprintf("notification:%s:%d", kind, n.ID), kind, resource, resourceID, expires)
	return nil
}
func (p *Publisher) PublishMessage(ctx context.Context, conv conversation.Conversation, _ string, m conversation.Message) {
	if p.sender == nil {
		return
	}
	work, ok := conv.(*conversation.WorkConversation)
	if !ok {
		return
	}
	var recipient int
	switch m.SenderRole {
	case conversation.SenderConsumer:
		recipient = work.ProviderID
	case conversation.SenderProvider:
		recipient = work.ConsumerID
	default:
		return
	}
	p.deliver(ctx, recipient, fmt.Sprintf("message:%d:%d", m.ID, recipient), "conversation.message.created", "conversation", conv.ID(), m.CreatedOn.Add(24*time.Hour))
}
func (p *Publisher) diagnose(kind, reason string) {
	slog.Warn("push delivery failed", "type", kind, "reason", reason)
}
func (p *Publisher) deliver(ctx context.Context, userID int, eventID, kind, resource string, resourceID int, expires time.Time) {
	ctx, cancel := context.WithTimeout(ctx, p.sender.timeout())
	defer cancel()
	if !expires.After(p.clock.Now()) {
		return
	}
	targets, err := p.installations.FindByUserID(ctx, userID)
	if err != nil {
		p.diagnose(kind, "installation_lookup")
		return
	}
	for _, target := range targets {
		if ctx.Err() != nil {
			p.diagnose(kind, "batch_timeout")
			break
		}
		ttl := expires.Sub(p.clock.Now())
		if ttl <= 0 {
			break
		}
		if ttl > 24*time.Hour {
			ttl = 24 * time.Hour
		}
		text := templates[kind][0]
		if target.Locale == "en" {
			text = templates[kind][1]
		}
		data := map[string]string{"version": "1", "event_id": eventID, "type": kind, "resource_type": resource, "resource_id": strconv.Itoa(resourceID), "destination": resource, "recipient_user_id": strconv.Itoa(userID), "recipient_app": target.App, "installation_id": target.ID, "binding_id": target.BindingID, "title": text[0], "body": text[1], "expires_at": expires.UTC().Format(time.RFC3339)}
		if err := p.sender.Send(ctx, target.Token, data, ttl); err != nil {
			if ctx.Err() != nil {
				p.diagnose(kind, "batch_timeout")
				break
			}
			if errors.Is(err, ErrInvalidToken) {
				target.Invalidate()
				if saveErr := p.installations.Save(ctx, &target); saveErr != nil {
					p.diagnose(kind, "installation_update")
				} else {
					p.diagnose(kind, "invalid_token")
				}
				continue
			}
			p.diagnose(kind, "fcm_send")
		}
	}
}
