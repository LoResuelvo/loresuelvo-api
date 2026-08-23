package steps_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	googlecalendar "github.com/LoResuelvo/loresuelvo-api/internal/adapters/google_calendar"
	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/cucumber/godog"
)

type calendarSyncRunner interface {
	RunOnce(context.Context) error
}

type calendarEventObserver interface {
	HasEventForUser(context.Context, int, int) (bool, error)
}

type calendarEventDetailsObserver interface {
	EventDetailsForUser(context.Context, int, int) (googlecalendar.EventDetails, bool, error)
}

type calendarEventCountObserver interface {
	EventCountForUser(context.Context, int, int) (int, error)
}

type calendarAvailabilityController interface {
	SetAvailable(bool)
}

func registerAddWorkOrderToCalendarSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una orden de trabajo futura para "([^"]*)" y "([^"]*)" el "([^"]*)" con una duración estimada de "([^"]*)" minutos y la descripción:$`, suite.thereIsFutureCalendarWorkOrder)
	sc.Step(`^que existe una orden de trabajo pasada para "([^"]*)" y "([^"]*)" el "([^"]*)" con una duración estimada de "([^"]*)" minutos y la descripción:$`, suite.thereIsPastCalendarWorkOrder)
	sc.Step(`^que existe una propuesta de servicio pendiente de "([^"]*)" para "([^"]*)" el "([^"]*)" con una duración estimada de "([^"]*)" minutos y la descripción:$`, suite.thereIsPendingCalendarConfirmationProposal)
	sc.Step(`^el consumidor "([^"]*)" tiene Google Calendar conectado$`, suite.consumerHasGoogleCalendarConnectedForWorkOrder)
	sc.Step(`^el prestador "([^"]*)" tiene Google Calendar conectado$`, suite.providerHasGoogleCalendarConnectedForWorkOrder)
	sc.Step(`^el consumidor "([^"]*)" no tiene Google Calendar conectado$`, suite.consumerDoesNotHaveGoogleCalendarConnection)
	sc.Step(`^el prestador "([^"]*)" no tiene Google Calendar conectado$`, suite.providerDoesNotHaveGoogleCalendarConnection)
	sc.Step(`^se sincronizan las órdenes de trabajo futuras con Google Calendar$`, suite.syncFutureWorkOrdersToCalendar)
	sc.Step(`^el consumidor "([^"]*)" recibe su cita en su Google Calendar$`, suite.consumerReceivesCalendarAppointment)
	sc.Step(`^el consumidor "([^"]*)" recibe la cita en su Google Calendar$`, suite.consumerReceivesCalendarAppointment)
	sc.Step(`^el consumidor "([^"]*)" recibe la cita futura en su Google Calendar$`, suite.consumerReceivesCalendarAppointment)
	sc.Step(`^el prestador "([^"]*)" recibe su cita en su Google Calendar$`, suite.providerReceivesCalendarAppointment)
	sc.Step(`^el consumidor "([^"]*)" no recibe una cita en Google Calendar$`, suite.consumerDoesNotReceiveCalendarAppointment)
	sc.Step(`^el consumidor "([^"]*)" no recibe una cita para ese turno$`, suite.consumerDoesNotReceiveCalendarAppointment)
	sc.Step(`^el prestador "([^"]*)" no recibe una cita en Google Calendar$`, suite.providerDoesNotReceiveCalendarAppointment)
	sc.Step(`^la cita del consumidor "([^"]*)" muestra el horario "([^"]*)", dura "([^"]*)" minutos, identifica a "([^"]*)", contiene la descripción y es privada$`, suite.consumerCalendarAppointmentShowsDetails)
	sc.Step(`^no se crea ninguna cita en Google Calendar$`, suite.noCalendarAppointmentIsCreated)
	sc.Step(`^el consumidor "([^"]*)" tiene una orden futura sin exportar$`, suite.consumerHasUnexportedFutureCalendarWorkOrder)
	sc.Step(`^el consumidor "([^"]*)" acaba de conectar Google Calendar$`, suite.consumerJustConnectedGoogleCalendar)
	sc.Step(`^Google Calendar no está disponible$`, suite.googleCalendarIsUnavailable)
	sc.Step(`^el consumidor "([^"]*)" tiene una cita pendiente por una indisponibilidad de Google Calendar$`, suite.consumerHasPendingCalendarAppointment)
	sc.Step(`^Google Calendar volvió a estar disponible$`, suite.googleCalendarBecameAvailable)
	sc.Step(`^se reintenta la sincronización de la cita pendiente$`, suite.retryPendingCalendarAppointmentSync)
	sc.Step(`^se sincroniza dos veces la misma orden de trabajo con Google Calendar$`, suite.syncSameCalendarWorkOrderTwice)
	sc.Step(`^el consumidor "([^"]*)" conserva una sola cita para ese turno$`, suite.consumerKeepsSingleCalendarAppointment)
	sc.Step(`^el consumidor "([^"]*)" recibe un aviso para volver a autorizar Google Calendar$`, suite.consumerReceivesCalendarReauthorizationNotice)
	sc.Step(`^se confirma la contratación de la propuesta$`, suite.confirmCalendarBooking)
	sc.Step(`^la contratación queda confirmada aunque no se pueda crear la cita$`, suite.calendarBookingIsConfirmedDespiteUnavailableCalendar)
}

func (suite *testSuite) thereIsFutureCalendarWorkOrder(consumerEmail, providerEmail, scheduledOn, duration string, description *godog.DocString) error {
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

func (suite *testSuite) thereIsPastCalendarWorkOrder(consumerEmail, providerEmail, scheduledOn, duration string, description *godog.DocString) error {
	if err := suite.prepareCalendarWorkOrderParticipants(providerEmail, consumerEmail); err != nil {
		return err
	}
	scheduledAt, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing past calendar work order scheduled_on: %w", err)
	}
	estimatedDuration, err := strconv.Atoi(duration)
	if err != nil {
		return fmt.Errorf("parsing past calendar work order duration: %w", err)
	}
	suite.clock.SetTime(scheduledAt.UTC().Add(-48 * time.Hour))
	amountCents, err := httphandler.ParseAmountToCents("15000.50")
	if err != nil {
		return fmt.Errorf("parsing past calendar work order amount: %w", err)
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

func (suite *testSuite) thereIsPendingCalendarConfirmationProposal(providerEmail, consumerEmail, scheduledOn, duration string, description *godog.DocString) error {
	if err := suite.prepareCalendarWorkOrderParticipants(providerEmail, consumerEmail); err != nil {
		return err
	}
	amountCents, err := httphandler.ParseAmountToCents("100000.00")
	if err != nil {
		return fmt.Errorf("parsing calendar confirmation amount: %w", err)
	}
	scheduledAt, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing calendar confirmation scheduled_on: %w", err)
	}
	estimatedDuration, err := strconv.Atoi(duration)
	if err != nil {
		return fmt.Errorf("parsing calendar confirmation duration: %w", err)
	}
	if err := suite.createServiceProposalFixture(
		providerEmail,
		consumerEmail,
		serviceproposal.StatusPending,
		amountCents,
		scheduledAt.UTC(),
		normalizeDocString(description),
		estimatedDuration,
	); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	return nil
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

func (suite *testSuite) consumerDoesNotHaveGoogleCalendarConnection(email string) error {
	return suite.participantDoesNotHaveGoogleCalendarConnection("consumer", email, auth0IDForConsumerEmail(email))
}

func (suite *testSuite) providerDoesNotHaveGoogleCalendarConnection(email string) error {
	return suite.participantDoesNotHaveGoogleCalendarConnection("provider", email, auth0IDForProviderEmail(email))
}

func (suite *testSuite) participantDoesNotHaveGoogleCalendarConnection(role, email, authID string) error {
	userID, err := suite.userRepository.FindIDByAuthID(authID)
	if err != nil {
		return fmt.Errorf("finding %s %q: %w", role, email, err)
	}
	_, err = suite.calendarConnectionRepository.FindByUserID(context.Background(), userID)
	if errors.Is(err, calendarconnection.ErrConnectionNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking Google Calendar connection for %s %q: %w", role, email, err)
	}
	return fmt.Errorf("expected %s %q not to have a Google Calendar connection", role, email)
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

func (suite *testSuite) googleCalendarIsUnavailable() error {
	if suite.calendarAvailability == nil {
		return fmt.Errorf("calendar availability controller is not configured")
	}
	suite.calendarAvailability.SetAvailable(false)
	return nil
}

func (suite *testSuite) consumerHasPendingCalendarAppointment(email string) error {
	if err := suite.googleCalendarIsUnavailable(); err != nil {
		return err
	}
	if suite.calendarSyncRunner == nil {
		return fmt.Errorf("calendar synchronization runner is not configured")
	}
	if err := suite.calendarSyncRunner.RunOnce(context.Background()); err == nil {
		return fmt.Errorf("expected calendar synchronization to fail while Calendar is unavailable")
	}
	if _, err := suite.userRepository.FindIDByAuthID(auth0IDForConsumerEmail(email)); err != nil {
		return fmt.Errorf("finding pending calendar appointment consumer %q: %w", email, err)
	}
	return nil
}

func (suite *testSuite) googleCalendarBecameAvailable() error {
	if suite.calendarAvailability == nil {
		return fmt.Errorf("calendar availability controller is not configured")
	}
	suite.calendarAvailability.SetAvailable(true)
	return nil
}

func (suite *testSuite) retryPendingCalendarAppointmentSync() error {
	if suite.calendarSyncRunner == nil {
		return fmt.Errorf("calendar synchronization runner is not configured")
	}
	return suite.calendarSyncRunner.RunOnce(context.Background())
}

func (suite *testSuite) syncSameCalendarWorkOrderTwice() error {
	if suite.calendarSyncRunner == nil {
		return fmt.Errorf("calendar synchronization runner is not configured")
	}
	if err := suite.calendarSyncRunner.RunOnce(context.Background()); err != nil {
		return fmt.Errorf("first calendar synchronization failed: %w", err)
	}
	if err := suite.calendarSyncRunner.RunOnce(context.Background()); err != nil {
		return fmt.Errorf("second calendar synchronization failed: %w", err)
	}
	return nil
}

func (suite *testSuite) consumerKeepsSingleCalendarAppointment(email string) error {
	observer, ok := suite.calendarEventObserver.(calendarEventCountObserver)
	if !ok {
		return fmt.Errorf("calendar event count observer is not configured")
	}
	userID, err := suite.userRepository.FindIDByAuthID(auth0IDForConsumerEmail(email))
	if err != nil {
		return fmt.Errorf("finding calendar appointment recipient %q: %w", email, err)
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	count, err := observer.EventCountForUser(context.Background(), userID, order.ID())
	if err != nil {
		return fmt.Errorf("counting calendar appointments for %q: %w", email, err)
	}
	if count != 1 {
		return fmt.Errorf("expected one calendar appointment for %q and work order %d, got %d", email, order.ID(), count)
	}
	return nil
}

func (suite *testSuite) consumerReceivesCalendarReauthorizationNotice(email string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}
	event, err := connection.readNotificationEvent(realtimeMessageTimeout)
	if err != nil {
		return fmt.Errorf("reading calendar reauthorization notification: %w", err)
	}
	if event.Type != "notification.created" {
		return fmt.Errorf("expected realtime event type %q, got %q", "notification.created", event.Type)
	}
	userID, err := suite.userRepository.FindIDByAuthID(auth0IDForConsumerEmail(email))
	if err != nil {
		return fmt.Errorf("finding calendar reauthorization recipient %q: %w", email, err)
	}
	created := event.Notification
	if created.UserID != userID {
		return fmt.Errorf("expected calendar reauthorization notification user %d, got %d", userID, created.UserID)
	}
	if created.Type != "calendar_reauthorization_required" {
		return fmt.Errorf("expected calendar reauthorization notification type, got %q", created.Type)
	}
	if created.ResourceType != "calendar_connection" || created.ResourceID != userID {
		return fmt.Errorf("expected calendar connection resource %d, got %s/%d", userID, created.ResourceType, created.ResourceID)
	}
	if created.ReadAt != nil || created.CreatedAt.IsZero() {
		return fmt.Errorf("expected unread calendar reauthorization notification with created_at")
	}
	return nil
}

func (suite *testSuite) confirmCalendarBooking() error {
	if suite.currentAuth0ID == "" {
		return fmt.Errorf("expected an authenticated consumer to confirm the booking")
	}
	if err := suite.startCheckoutForPreparedProposal(); err != nil {
		return err
	}
	return suite.processApprovedPayment()
}

func (suite *testSuite) calendarBookingIsConfirmedDespiteUnavailableCalendar() error {
	if err := suite.serviceProposalIsAccepted(); err != nil {
		return err
	}
	return suite.systemRegistersOneScheduledWorkOrder()
}

func (suite *testSuite) consumerReceivesCalendarAppointment(email string) error {
	return suite.participantReceivesCalendarAppointment(email, auth0IDForConsumerEmail(email))
}

func (suite *testSuite) providerReceivesCalendarAppointment(email string) error {
	return suite.participantReceivesCalendarAppointment(email, auth0IDForProviderEmail(email))
}

func (suite *testSuite) consumerCalendarAppointmentShowsDetails(email, scheduledOn, duration, counterpartName string) error {
	if suite.calendarEventDetailsObserver == nil {
		return fmt.Errorf("calendar event details observer is not configured")
	}
	userID, err := suite.userRepository.FindIDByAuthID(auth0IDForConsumerEmail(email))
	if err != nil {
		return fmt.Errorf("finding calendar appointment recipient %q: %w", email, err)
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	expectedStart, err := time.Parse(time.RFC3339, scheduledOn)
	if err != nil {
		return fmt.Errorf("parsing expected calendar appointment start: %w", err)
	}
	expectedDuration, err := strconv.Atoi(duration)
	if err != nil {
		return fmt.Errorf("parsing expected calendar appointment duration: %w", err)
	}
	details, found, err := suite.calendarEventDetailsObserver.EventDetailsForUser(
		context.Background(),
		userID,
		order.ID(),
	)
	if err != nil {
		return fmt.Errorf("finding calendar appointment details for %q: %w", email, err)
	}
	if !found {
		return fmt.Errorf("expected calendar appointment for %q and work order %d", email, order.ID())
	}
	if !details.Start.Equal(expectedStart.UTC()) {
		return fmt.Errorf("expected calendar appointment to start at %s, got %s", expectedStart.UTC(), details.Start.UTC())
	}
	if details.End.Sub(details.Start) != time.Duration(expectedDuration)*time.Minute {
		return fmt.Errorf("expected calendar appointment duration of %d minutes, got %s", expectedDuration, details.End.Sub(details.Start))
	}
	if !strings.Contains(details.Description, counterpartName) {
		return fmt.Errorf("expected calendar appointment to identify counterpart %q", counterpartName)
	}
	if !strings.Contains(details.Description, order.Description()) {
		return fmt.Errorf("expected calendar appointment to contain work order description")
	}
	if details.Visibility != "private" {
		return fmt.Errorf("expected calendar appointment to be private, got %q", details.Visibility)
	}
	return nil
}

func (suite *testSuite) consumerDoesNotReceiveCalendarAppointment(email string) error {
	return suite.participantDoesNotReceiveCalendarAppointment(email, auth0IDForConsumerEmail(email))
}

func (suite *testSuite) providerDoesNotReceiveCalendarAppointment(email string) error {
	return suite.participantDoesNotReceiveCalendarAppointment(email, auth0IDForProviderEmail(email))
}

func (suite *testSuite) participantDoesNotReceiveCalendarAppointment(email, authID string) error {
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
	if hasEvent {
		return fmt.Errorf("expected no calendar appointment for %q and work order %d", email, order.ID())
	}
	return nil
}

func (suite *testSuite) noCalendarAppointmentIsCreated() error {
	if suite.calendarEventObserver == nil {
		return fmt.Errorf("calendar event observer is not configured")
	}
	order, err := suite.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	for _, participant := range []user.User{order.Consumer(), order.Provider()} {
		userID := participant.ID()
		hasEvent, err := suite.calendarEventObserver.HasEventForUser(context.Background(), userID, order.ID())
		if err != nil {
			return fmt.Errorf("checking absent calendar appointment: %w", err)
		}
		if hasEvent {
			return fmt.Errorf("expected no calendar appointment for user %d and work order %d", userID, order.ID())
		}
	}
	return nil
}

func (suite *testSuite) consumerHasUnexportedFutureCalendarWorkOrder(email string) error {
	return suite.participantDoesNotReceiveCalendarAppointment(email, auth0IDForConsumerEmail(email))
}

func (suite *testSuite) consumerJustConnectedGoogleCalendar(email string) error {
	return suite.connectGoogleCalendarForWorkOrderParticipant(email, auth0IDForConsumerEmail(email))
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
