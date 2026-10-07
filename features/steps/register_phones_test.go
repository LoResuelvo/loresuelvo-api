package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type phoneRegistration struct {
	Secret            string `json:"installation_secret"`
	App               string `json:"app"`
	Token             string `json:"fcm_token"`
	Locale            string `json:"locale,omitempty"`
	BindingID         string `json:"binding_id"`
	PreviousBindingID string `json:"previous_binding_id,omitempty"`
}

func registerPhoneSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que Ana registró dos teléfonos para recibir avisos de su cuenta$`, s.anaRegisteredTwoPhones)
	sc.Step(`^Juan le envía un mensaje$`, func() error { return s.noticeMessage("Juan", "de texto") })
	sc.Step(`^LoResuelvo envía el aviso de nuevo mensaje a los dos teléfonos de Ana$`, func() error { return s.assertPhoneNotices(2) })
	sc.Step(`^que desvinculó uno de ellos$`, s.anaRemovedPhone)
	sc.Step(`^LoResuelvo envía el aviso solamente al teléfono que sigue registrado$`, func() error { return s.assertPhoneNotices(1) })
	sc.Step(`^que Ana tenía un teléfono registrado para recibir sus avisos$`, s.anaRegisteredOnePhone)
	sc.Step(`^que Carla inició sesión en ese mismo teléfono y lo registró para su cuenta$`, s.carlaRegisteredPhone)
	sc.Step(`^Juan le envía un mensaje a Ana$`, func() error { return s.noticeMessage("Juan", "de texto") })
	sc.Step(`^LoResuelvo no envía el aviso de Ana al teléfono que ahora usa Carla$`, func() error {
		if len(s.pushCapture.snapshot()) != 0 {
			return fmt.Errorf("unexpected former account push")
		}
		return nil
	})
}
func (s *testSuite) registerPhone(id, authID string, request phoneRegistration) error {
	if err := s.phoneRequest(http.MethodPut, id, authID, request); err != nil {
		return err
	}
	if s.lastStatus != http.StatusCreated && s.lastStatus != http.StatusOK {
		return fmt.Errorf("installation registration returned status %d", s.lastStatus)
	}
	return nil
}
func (s *testSuite) phoneRequest(method, id, authID string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(method, s.server.URL+"/installations/"+id, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if authID != "" {
		request.Header.Set("Authorization", "Bearer "+s.tokenBuilder.BuildToken(authID, nil))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("installation API connection failed: %w", err)
	}
	defer response.Body.Close()
	s.lastStatus = response.StatusCode
	s.lastBody, err = io.ReadAll(response.Body)
	return err
}
func (s *testSuite) anaRegisteredOnePhone() error {
	s.phoneIDs = []string{uuid.NewString()}
	s.phoneRegistrations = []phoneRegistration{{Secret: uuid.NewString(), App: "consumer", Token: "ana-device-one", BindingID: uuid.NewString()}}
	return s.registerPhone(s.phoneIDs[0], auth0IDForConsumerEmail("ana@example.com"), s.phoneRegistrations[0])
}
func (s *testSuite) anaRegisteredTwoPhones() error {
	if err := s.anaRegisteredOnePhone(); err != nil {
		return err
	}
	s.phoneIDs = append(s.phoneIDs, uuid.NewString())
	s.phoneRegistrations = append(s.phoneRegistrations, phoneRegistration{Secret: uuid.NewString(), App: "consumer", Token: "ana-device-two", BindingID: uuid.NewString()})
	return s.registerPhone(s.phoneIDs[1], auth0IDForConsumerEmail("ana@example.com"), s.phoneRegistrations[1])
}
func (s *testSuite) anaRemovedPhone() error {
	r := s.phoneRegistrations[0]
	if err := s.phoneRequest(http.MethodDelete, s.phoneIDs[0], auth0IDForConsumerEmail("ana@example.com"), map[string]string{"installation_secret": r.Secret, "binding_id": r.BindingID}); err != nil {
		return err
	}
	return s.lastResponseShouldHaveStatusCode(http.StatusNoContent)
}
func (s *testSuite) carlaRegisteredPhone() error {
	if err := s.thereIsRegisteredConsumerWithEmailNameAndSurname("carla@example.com", "Carla", "Lopez"); err != nil {
		return err
	}
	r := s.phoneRegistrations[0]
	r.PreviousBindingID = r.BindingID
	r.BindingID = uuid.NewString()
	return s.registerPhone(s.phoneIDs[0], auth0IDForConsumerEmail("carla@example.com"), r)
}
func (s *testSuite) assertPhoneNotices(count int) error {
	requests := s.pushCapture.snapshot()
	if len(requests) != count {
		return fmt.Errorf("expected %d phone notices, got %d", count, len(requests))
	}
	seen := map[string]bool{}
	eventID := ""
	for _, r := range requests {
		expected := -1
		for index, registration := range s.phoneRegistrations {
			if registration.Token == r.Message.Token {
				expected = index
			}
		}
		if expected < 0 || seen[r.Message.Token] || (count == 1 && expected != 1) {
			return fmt.Errorf("unexpected phone target")
		}
		seen[r.Message.Token] = true
		if r.Message.Data["type"] != "conversation.message.created" || r.Message.Data["installation_id"] != s.phoneIDs[expected] || r.Message.Data["binding_id"] != s.phoneRegistrations[expected].BindingID {
			return fmt.Errorf("incorrect phone notice binding")
		}
		if eventID != "" && eventID != r.Message.Data["event_id"] {
			return fmt.Errorf("event must keep same ID across devices")
		}
		eventID = r.Message.Data["event_id"]
	}
	return nil
}
