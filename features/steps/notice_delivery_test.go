package steps_test

import (
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

func registerNoticeDeliverySteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que Juan tiene un teléfono registrado para recibir avisos$`, func() error { return s.noticePhone("Juan", "es") })
	sc.Step(`^que el servicio de avisos al teléfono no está disponible$`, func() error { s.pushCapture.setStatus(http.StatusServiceUnavailable); return nil })
	sc.Step(`^Ana le envía un mensaje$`, func() error { return s.noticeMessage("Ana", "de texto") })
	sc.Step(`^LoResuelvo le confirma a Ana que el mensaje fue enviado$`, func() error {
		if err := s.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
			return err
		}
		message, err := s.sentMessageResponseFromLastBody()
		if err != nil {
			return err
		}
		s.lastSentMessageID = message.ID
		if len(s.pushCapture.snapshot()) != 1 {
			return fmt.Errorf("expected a real failed FCM attempt")
		}
		return nil
	})
	sc.Step(`^Juan puede consultar el mensaje en la conversación$`, func() error {
		s.currentAuth0ID = auth0IDForProviderEmail("juan.plomero@example.com")
		if err := s.requestConversationByID(s.lastConversationID); err != nil {
			return err
		}
		if err := s.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
			return err
		}
		detail, err := s.conversationDetailResponseFromLastBody()
		if err != nil {
			return err
		}
		for _, message := range detail.Messages {
			if message.ID == s.lastSentMessageID && message.Content == "Hello" && message.SenderRole == "consumer" {
				return nil
			}
		}
		return fmt.Errorf("persisted message not queryable by recipient after FCM failure")
	})
}
