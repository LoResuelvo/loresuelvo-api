package steps_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/cucumber/godog"
)

type calendarSyncRunner interface {
	RunOnce(context.Context) error
}

type calendarEventObserver interface {
	HasEventForUser(context.Context, int, int) (bool, error)
}

func registerAddWorkOrderToCalendarSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una orden de trabajo futura para "([^"]*)" y "([^"]*)" el "([^"]*)" con una duración estimada de "([^"]*)" minutos y la descripción:$`, suite.thereIsFutureCalendarWorkOrder)
	sc.Step(`^el consumidor "([^"]*)" tiene Google Calendar conectado$`, suite.consumerHasGoogleCalendarConnectedForWorkOrder)
	sc.Step(`^el prestador "([^"]*)" tiene Google Calendar conectado$`, suite.providerHasGoogleCalendarConnectedForWorkOrder)
	sc.Step(`^se sincronizan las órdenes de trabajo futuras con Google Calendar$`, suite.syncFutureWorkOrdersToCalendar)
	sc.Step(`^el consumidor "([^"]*)" recibe su cita en su Google Calendar$`, suite.consumerReceivesCalendarAppointment)
	sc.Step(`^el prestador "([^"]*)" recibe su cita en su Google Calendar$`, suite.providerReceivesCalendarAppointment)
}

func (suite *testSuite) thereIsFutureCalendarWorkOrder(providerEmail, consumerEmail, scheduledOn, duration string, description *godog.DocString) error {
	if err := suite.prepareCalendarWorkOrderParticipants(providerEmail, consumerEmail); err != nil {
		return err
	}
	amountCents, err := httphandler.ParseAmountToCents("15000.50")
	if err != nil {
		return fmt.Errorf("parsing calendar work order amount: %w", err)
	}
	scheduledAt, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing calendar work order scheduled_on: %w", err)
	}
	estimatedDuration, err := strconv.Atoi(duration)
	if err != nil {
		return fmt.Errorf("parsing calendar work order duration: %w", err)
	}
	return suite.createScheduledWorkOrderFixtureWithDuration(
		providerEmail,
		consumerEmail,
		amountCents,
		scheduledAt.UTC(),
		normalizeDocString(description),
		estimatedDuration,
	)
}

func (suite *testSuite) prepareCalendarWorkOrderParticipants(providerEmail, consumerEmail string) error {
	if err := suite.thereIsCategoryNamed("Plomería"); err != nil {
		return fmt.Errorf("preparing calendar work order category: %w", err)
	}
	if _, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(consumerEmail)); err != nil {
		if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(consumerEmail, "Ana", "Pérez"); err != nil {
			return fmt.Errorf("preparing calendar work order consumer: %w", err)
		}
	}
	if _, err := suite.userRepository.FindByAuthID(auth0IDForProviderEmail(providerEmail)); err != nil {
		if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(providerEmail, "Juan", "Gómez", "Plomería"); err != nil {
			return fmt.Errorf("preparing calendar work order provider: %w", err)
		}
	}
	return nil
}

func (suite *testSuite) consumerHasGoogleCalendarConnectedForWorkOrder(email string) error {
	return suite.connectGoogleCalendarForWorkOrderParticipant(email, auth0IDForConsumerEmail(email))
}

func (suite *testSuite) providerHasGoogleCalendarConnectedForWorkOrder(email string) error {
	return suite.connectGoogleCalendarForWorkOrderParticipant(email, auth0IDForProviderEmail(email))
}

func (suite *testSuite) connectGoogleCalendarForWorkOrderParticipant(email, authID string) error {
	previousAuthID := suite.currentAuth0ID
	suite.currentAuth0ID = authID
	defer func() { suite.currentAuth0ID = previousAuthID }()
	if err := suite.startGoogleCalendarWebAuthorization(); err != nil {
		return err
	}
	if err := suite.systemReturnsGoogleCalendarWebAuthorization(); err != nil {
		return fmt.Errorf("starting Google Calendar connection for %q: %w", email, err)
	}
	if err := suite.authorizeGoogleCalendarAccess(); err != nil {
		return err
	}
	return suite.systemConfirmsGoogleCalendarConnection()
}

func (suite *testSuite) syncFutureWorkOrdersToCalendar() error {
	if suite.calendarSyncRunner == nil {
		return fmt.Errorf("calendar synchronization runner is not configured")
	}
	return suite.calendarSyncRunner.RunOnce(context.Background())
}

func (suite *testSuite) consumerReceivesCalendarAppointment(email string) error {
	return suite.participantReceivesCalendarAppointment(email, auth0IDForConsumerEmail(email))
}

func (suite *testSuite) providerReceivesCalendarAppointment(email string) error {
	return suite.participantReceivesCalendarAppointment(email, auth0IDForProviderEmail(email))
}

func (suite *testSuite) participantReceivesCalendarAppointment(email, authID string) error {
	if suite.calendarEventObserver == nil {
		return fmt.Errorf("calendar event observer is not configured")
	}
	userID, err := suite.userRepository.FindIDByAuthID(authID)
	if err != nil {
		return fmt.Errorf("finding calendar appointment recipient %q: %w", email, err)
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	hasEvent, err := suite.calendarEventObserver.HasEventForUser(context.Background(), userID, order.ID())
	if err != nil {
		return fmt.Errorf("checking calendar appointment for %q: %w", email, err)
	}
	if !hasEvent {
		return fmt.Errorf("expected calendar appointment for %q and work order %d", email, order.ID())
	}
	return nil
}
