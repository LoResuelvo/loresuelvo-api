package steps_test

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
)

func registerNotifyUrgentWorkOrdersSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^el scheduler revisa las órdenes de trabajo urgentes$`, suite.schedulerChecksUrgentWorkOrders)
	sc.Step(`^el consumidor "([^"]*)" recibe la notificación de orden de trabajo urgente$`, suite.participantReceivesUrgentWorkOrderNotification)
	sc.Step(`^el prestador "([^"]*)" recibe la notificación de orden de trabajo urgente$`, suite.participantReceivesUrgentWorkOrderNotification)
	sc.Step(`^el consumidor "([^"]*)" no recibe notificaciones de órdenes de trabajo urgentes$`, suite.participantDoesNotReceiveUrgentWorkOrderNotifications)
	sc.Step(`^el prestador "([^"]*)" no recibe notificaciones de órdenes de trabajo urgentes$`, suite.participantDoesNotReceiveUrgentWorkOrderNotifications)
}

func (suite *testSuite) schedulerChecksUrgentWorkOrders() error {
	return suite.urgentWorkOrderScheduler.RunOnce(context.Background())
}

func (suite *testSuite) participantReceivesUrgentWorkOrderNotification(email string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}
	event, err := connection.readNotificationEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}
	workOrder, err := suite.workOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	expectedUserID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("finding urgent notification recipient: %w", err)
	}
	if event.Type != "notification.created" {
		return fmt.Errorf("expected realtime event type %q, got %q", "notification.created", event.Type)
	}
	created := event.Notification
	if created.ID == 0 {
		return fmt.Errorf("expected persisted urgent work order notification id")
	}
	if created.UserID != expectedUserID {
		return fmt.Errorf("expected notification user_id %d, got %d", expectedUserID, created.UserID)
	}
	if created.Type != "work_order_close_to_scheduled_time" {
		return fmt.Errorf("expected urgent work order notification type, got %q", created.Type)
	}
	if created.ResourceType != "work_order" {
		return fmt.Errorf("expected notification resource_type %q, got %q", "work_order", created.ResourceType)
	}
	if created.ResourceID != workOrder.ID {
		return fmt.Errorf("expected notification resource_id %d, got %d", workOrder.ID, created.ResourceID)
	}
	if created.ReadAt != nil || created.CreatedAt.IsZero() {
		return fmt.Errorf("expected a persisted unread notification with created_at")
	}
	return nil
}

func (suite *testSuite) participantDoesNotReceiveUrgentWorkOrderNotifications(email string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}
	event, err := connection.readNotificationEvent(realtimeMessageTimeout)
	if err == nil {
		return fmt.Errorf("expected no urgent work order notification for %q, got %+v", email, event)
	}
	if isRealtimeReadTimeout(err) {
		return nil
	}
	return err
}
