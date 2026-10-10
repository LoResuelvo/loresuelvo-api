package steps_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net/http"
	"testing"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
)

type jobRequestNoticeData struct {
	ID             int    `json:"id"`
	ConversationID int    `json:"conversation_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	Requester      struct {
		Name    string `json:"name"`
		Surname string `json:"surname"`
	} `json:"requester"`
	Images []messageImageResponse `json:"images"`
}

func registerNotifyJobRequestSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real la notificación de una nueva solicitud de trabajo$`, s.providerReceivesJobRequestNotification)
	sc.Step(`^la notificación identifica la solicitud de trabajo enviada$`, s.jobRequestNoticeIdentifiesRequest)
	sc.Step(`^la notificación incluye los datos del consumidor, el título y la descripción de la solicitud$`, s.jobRequestNoticeIncludesDetails)
	sc.Step(`^la notificación incluye las imágenes adjuntas a la solicitud$`, s.jobRequestNoticeIncludesImages)
	sc.Step(`^el consumidor "([^"]*)" no recibe la notificación de su propia solicitud de trabajo$`, s.participantDoesNotReceiveRealtimeMessages)
	sc.Step(`^el prestador "([^"]*)" no recibe la notificación de esa solicitud de trabajo$`, s.participantDoesNotReceiveRealtimeMessages)
	sc.Step(`^el sistema envía solamente a Juan un aviso de nueva solicitud de trabajo$`, s.assertJobRequestPhoneNotice)
	sc.Step(`^el aviso indica cuál es la solicitud recibida$`, s.assertJobRequestPhoneNotice)
	sc.Step(`^el aviso no incluye los datos del consumidor, el título, la descripción ni enlaces a las imágenes adjuntas$`, s.assertJobRequestPhonePrivacy)
}

func (s *testSuite) providerReceivesJobRequestNotification(email string) error {
	if err := s.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	connection, err := s.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}
	event, err := connection.readNotificationEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}
	id, err := s.providerIDByEmail(email)
	if err != nil {
		return err
	}
	n := event.Notification
	if event.Type != "notification.created" || n.ID <= 0 || n.UserID != id || n.Type != "job_request_received" || n.ResourceType != "job_request" {
		return fmt.Errorf("unexpected job request notification: %+v", event)
	}
	s.lastJobRequestNotice = &event
	return nil
}

func (s *testSuite) jobRequestNoticeIdentifiesRequest() error {
	response, err := s.jobRequestCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	event := s.lastJobRequestNotice
	if event == nil || event.JobRequest == nil || event.Notification.ResourceID != response.ID || event.JobRequest.ID != response.ID || event.JobRequest.ConversationID != response.ConversationID || event.JobRequest.Status != response.Status {
		return fmt.Errorf("notification does not identify created job request")
	}
	return nil
}

func (s *testSuite) jobRequestNoticeIncludesDetails() error {
	response, err := s.jobRequestCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	if err := s.jobRequestNoticeIdentifiesRequest(); err != nil {
		return err
	}
	request := s.lastJobRequestNotice.JobRequest
	if request.Requester.Name != "Ana" || request.Requester.Surname != "Gomez" || request.Title != response.Title || request.Description != response.Description {
		return fmt.Errorf("notification job request details mismatch: %+v", request)
	}
	return nil
}

func (s *testSuite) jobRequestNoticeIncludesImages() error {
	if err := s.jobRequestNoticeIdentifiesRequest(); err != nil {
		return err
	}
	return s.assertMessageImages(s.lastJobRequestNotice.JobRequest.Images, s.lastAttemptedMessageImageNames)
}

func (s *testSuite) assertJobRequestPhoneNotice() error {
	if err := s.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	response, err := s.jobRequestCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	return s.assertNotice([]string{"Juan"}, "job_request_received", "job_request", response.ID)
}

func (s *testSuite) assertJobRequestPhonePrivacy() error {
	if err := s.assertJobRequestPhoneNotice(); err != nil {
		return err
	}
	data := s.pushCapture.snapshot()[0].Message.Data
	if data["title"] != "Nueva solicitud de trabajo" || data["body"] != "Recibiste una solicitud de trabajo." {
		return fmt.Errorf("unexpected job request phone preview")
	}
	allowed := map[string]bool{"version": true, "event_id": true, "type": true, "resource_type": true, "resource_id": true, "destination": true, "recipient_user_id": true, "recipient_app": true, "installation_id": true, "binding_id": true, "title": true, "body": true, "expires_at": true}
	for key := range data {
		if !allowed[key] {
			return fmt.Errorf("unexpected private phone notice field %q", key)
		}
	}
	return nil
}

func TestRealtimeReaderReassemblesFragmentedNotification(t *testing.T) {
	first := bytes.Repeat([]byte("a"), 1024)
	second := []byte("rest of notification")
	var wire bytes.Buffer
	wire.Write([]byte{0x01, 126})
	require.NoError(t, binary.Write(&wire, binary.BigEndian, uint16(len(first))))
	wire.Write(first)
	wire.Write([]byte{0x00, 0}) // non-final empty continuation is valid
	wire.Write([]byte{0x80, byte(len(second))})
	wire.Write(second)
	connection := &realtimeTestConnection{reader: bufio.NewReader(&wire)}
	payload, err := connection.readTextFrame()
	require.NoError(t, err)
	require.Equal(t, append(first, second...), payload)
}

func TestRealtimeReaderRejectsTextFrameInsteadOfContinuation(t *testing.T) {
	wire := []byte{0x01, 1, 'a', 0x81, 1, 'b'}
	connection := &realtimeTestConnection{reader: bufio.NewReader(bytes.NewReader(wire))}
	_, err := connection.readTextFrame()
	require.ErrorContains(t, err, "expected realtime text frame")
}
