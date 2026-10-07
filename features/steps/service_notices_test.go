package steps_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/cucumber/godog"
)

func registerServiceNoticeSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que Ana es una consumidora y Juan es un prestador registrados$`, s.noticeParticipants)
	sc.Step(`^que ambos tienen un teléfono registrado para recibir avisos$`, s.noticePhones)
	sc.Step(`^que Ana y Juan pueden conversar sobre un trabajo$`, s.noticeConversation)
	sc.Step(`^que (Ana|Juan) no está usando la aplicación$`, func(name string) error {
		if len(s.realtimeConnections) > 0 {
			return fmt.Errorf("expected no realtime connections")
		}
		return nil
	})
	sc.Step(`^(Ana|Juan) envía un mensaje (de texto|con varias imágenes|de audio|con video)$`, s.noticeMessage)
	sc.Step(`^LoResuelvo envía al teléfono de (Ana|Juan) un aviso de nuevo mensaje que indica a qué conversación pertenece$`, func(name string) error {
		return s.assertNotice([]string{name}, "conversation.message.created", "conversation", s.lastConversationID)
	})
	sc.Step(`^no envía ese aviso al autor ni a personas ajenas a la conversación$`, func() error {
		if len(s.pushCapture.snapshot()) != 1 {
			return fmt.Errorf("expected exactly one message notice")
		}
		return nil
	})
	sc.Step(`^que Juan puede ofrecerle un servicio a Ana$`, s.noticeProposalReady)
	sc.Step(`^Juan le envía una propuesta válida$`, s.noticeProposal)
	sc.Step(`^LoResuelvo envía solamente a Ana un aviso de nueva propuesta$`, func() error {
		return s.assertNotice([]string{"Ana"}, "service_proposal_received", "service_proposal", s.lastServiceProposalID)
	})
	sc.Step(`^el aviso indica cuál es la propuesta recibida$`, func() error {
		return s.assertNotice([]string{"Ana"}, "service_proposal_received", "service_proposal", s.lastServiceProposalID)
	})
	sc.Step(`^que Ana inició el pago de la seña de una propuesta vigente de Juan$`, s.noticeDepositReady)
	sc.Step(`^LoResuelvo verifica que la seña fue aprobada y confirma la contratación$`, func() error { s.pushCapture.reset(); return s.processApprovedPayment() })
	sc.Step(`^LoResuelvo envía solamente a Juan un aviso de propuesta aceptada$`, func() error { return s.assertOrderNotice("Juan", "service_proposal_accepted") })
	sc.Step(`^el aviso indica cuál es la orden de trabajo generada$`, func() error { return s.assertOrderNotice("Juan", "service_proposal_accepted") })
	sc.Step(`^que Ana y Juan tienen un trabajo programado para dentro de 23 horas$`, func() error { return s.noticeScheduledOrder(23 * time.Hour) })
	sc.Step(`^LoResuelvo revisa los próximos turnos más de una vez$`, func() error {
		s.pushCapture.reset()
		if err := s.schedulerChecksUrgentWorkOrders(); err != nil {
			return err
		}
		return s.schedulerChecksUrgentWorkOrders()
	})
	sc.Step(`^envía a Ana y a Juan un único recordatorio a cada uno sobre ese trabajo$`, s.assertNoticeReminder)
	sc.Step(`^que Juan tiene un trabajo de Ana pendiente de finalizar$`, func() error { return s.noticeScheduledOrder(48 * time.Hour) })
	sc.Step(`^que ya llegó la fecha acordada para realizarlo$`, s.noticeAtScheduledTime)
	sc.Step(`^Juan informa que terminó el trabajo con una descripción y fotos válidas$`, s.noticeCompletion)
	sc.Step(`^LoResuelvo envía solamente a Ana un aviso de trabajo finalizado$`, func() error { return s.assertOrderNotice("Ana", "work_order_completion_reported") })
	sc.Step(`^el aviso indica cuál es el trabajo que puede revisar$`, func() error { return s.assertOrderNotice("Ana", "work_order_completion_reported") })
	sc.Step(`^que Juan informó la finalización del trabajo de Ana con las fotos requeridas$`, s.noticeCompletedOrder)
	sc.Step(`^que Ana inició el pago del saldo de ese trabajo$`, func() error { return s.consumerStartedServiceBalanceCheckout("ana@example.com") })
	sc.Step(`^LoResuelvo verifica que el pago fue aprobado y registra el trabajo como pagado$`, s.noticeFinalPayment)
	sc.Step(`^LoResuelvo envía solamente a Juan un aviso de pago final confirmado$`, func() error { return s.assertOrderNotice("Juan", "work_order_final_payment_approved") })
	sc.Step(`^el aviso indica cuál es el trabajo pagado$`, func() error {
		if err := s.workOrderIsFullyPaid(); err != nil {
			return err
		}
		return s.assertOrderNotice("Juan", "work_order_final_payment_approved")
	})
	sc.Step(`^que Ana inició el pago del saldo de un trabajo realizado por Juan$`, s.noticeBalanceReady)
	sc.Step(`^el pago queda pendiente$`, func() error { s.pushCapture.reset(); return s.processNonApprovedServiceBalancePayment("en proceso") })
	sc.Step(`^el pago es rechazado$`, func() error { s.pushCapture.reset(); return s.processNonApprovedServiceBalancePayment("rechazado") })
	sc.Step(`^Ana vuelve del sitio de pago sin que se verifique el cobro$`, func() error {
		s.pushCapture.reset()
		return s.serviceBalanceIntentCanBeReadWithStatus("checkout_ready")
	})
	sc.Step(`^LoResuelvo no envía un aviso de pago final confirmado$`, s.assertNoFinalNotice)
	sc.Step(`^que LoResuelvo ya confirmó el pago final de un trabajo de Juan y envió su aviso$`, func() error {
		if err := s.noticeBalanceReady(); err != nil {
			return err
		}
		if err := s.noticeFinalPayment(); err != nil {
			return err
		}
		return s.assertOrderNotice("Juan", "work_order_final_payment_approved")
	})
	sc.Step(`^vuelve a recibir la confirmación de ese mismo pago$`, func() error {
		s.pushCapture.reset()
		return s.sendMercadoPagoPaymentNotification(s.lastExternalPaymentID)
	})
	sc.Step(`^no envía otro aviso de pago final confirmado$`, s.assertNoFinalNotice)
	sc.Step(`^que Juan todavía no aceptó la solicitud de trabajo de Ana$`, func() error {
		return s.createPendingConversationBetweenConsumerAndProvider("ana@example.com", "juan.plomero@example.com", "Request for service")
	})
	sc.Step(`^Juan intenta enviarle un mensaje en esa conversación$`, func() error {
		s.pushCapture.reset()
		s.currentAuth0ID = auth0IDForProviderEmail("juan.plomero@example.com")
		return s.requestSendMessageToPreparedConversation(sendMessageRequest{Content: "Hello"})
	})
	sc.Step(`^LoResuelvo rechaza el mensaje$`, func() error { return s.lastResponseShouldHaveStatusCode(http.StatusForbidden) })
	sc.Step(`^no envía a Ana un aviso de nuevo mensaje$`, func() error {
		if len(s.pushCapture.snapshot()) != 0 {
			return fmt.Errorf("unexpected push for rejected message")
		}
		return nil
	})
	sc.Step(`^Ana envía un mensaje con su nombre, dirección y fotos del domicilio$`, func() error { return s.noticeMessage("Ana", "private") })
	sc.Step(`^el aviso para Juan solo informa que tiene un nuevo mensaje en LoResuelvo$`, func() error {
		if err := s.assertNotice([]string{"Juan"}, "conversation.message.created", "conversation", s.lastConversationID); err != nil {
			return err
		}
		r := s.pushCapture.snapshot()[0]
		if r.Message.Data["title"] != "Nuevo mensaje" || r.Message.Data["body"] != "Tenés un nuevo mensaje en LoResuelvo." {
			return fmt.Errorf("unsafe message preview")
		}
		return nil
	})
	sc.Step(`^no incluye el contenido del mensaje ni los enlaces a las fotos$`, func() error {
		r := s.pushCapture.snapshot()[0]
		allowed := map[string]bool{"version": true, "event_id": true, "type": true, "resource_type": true, "resource_id": true, "destination": true, "recipient_user_id": true, "recipient_app": true, "installation_id": true, "binding_id": true, "title": true, "body": true, "expires_at": true}
		for key := range r.Message.Data {
			if !allowed[key] {
				return fmt.Errorf("unexpected private push field %q", key)
			}
		}
		return nil
	})
	sc.Step(`^que el teléfono de Ana está registrado (en español|en inglés|sin elegir un idioma)$`, s.noticeLocale)
	sc.Step(`^el aviso de nueva propuesta para Ana está escrito en (español|inglés)$`, s.assertNoticeLocale)
}

func (s *testSuite) noticeParticipants() error {
	if err := s.systemDateTimeIs("2026-07-01T12:00:00Z"); err != nil {
		return err
	}
	if err := s.thereIsCategoryNamed("Plomería"); err != nil {
		return err
	}
	if err := s.thereIsRegisteredConsumerWithEmailNameAndSurname("ana@example.com", "Ana", "Gomez"); err != nil {
		return err
	}
	return s.thereIsRegisteredProviderWithEmailNameSurnameAndCategory("juan.plomero@example.com", "Juan", "Perez", "Plomería")
}
func noticeEmail(name string) string {
	if name == "Ana" {
		return "ana@example.com"
	}
	return "juan.plomero@example.com"
}
func (s *testSuite) noticePhone(name, locale string) error {
	id, err := s.userRepository.FindIDByEmail(noticeEmail(name))
	if err != nil {
		return err
	}
	app := "consumer"
	if name == "Juan" {
		app = "provider"
	}
	i, err := installation.New("phone-"+name, id, app, "token-"+name, locale, "binding-"+name)
	if err != nil {
		return err
	}
	existing, findErr := s.dependencies.Persistence.InstallationRepository.FindByID(context.Background(), i.ID)
	if findErr == nil {
		i.Revision = existing.Revision
		i.SecretHash = existing.SecretHash
	} else if !errors.Is(findErr, installation.ErrNotFound) {
		return findErr
	}
	return s.dependencies.Persistence.InstallationRepository.Save(context.Background(), i)
}
func (s *testSuite) noticePhones() error {
	if err := s.noticePhone("Ana", "es"); err != nil {
		return err
	}
	return s.noticePhone("Juan", "es")
}
func (s *testSuite) noticeConversation() error {
	if _, err := s.userRepository.FindIDByEmail("ana@example.com"); errors.Is(err, sql.ErrNoRows) {
		if err := s.noticeParticipants(); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return s.thereIsActiveChatBetweenConsumerAndProvider("ana@example.com", "juan.plomero@example.com")
}
func (s *testSuite) noticeProposalReady() error {
	if err := s.noticeConversation(); err != nil {
		return err
	}
	return s.prepareLinkedMercadoPagoAccount("mp-juan", "juan.plomero@example.com")
}
func (s *testSuite) noticeProposal() error {
	s.pushCapture.reset()
	s.currentAuth0ID = auth0IDForProviderEmail("juan.plomero@example.com")
	if err := s.sendServiceProposalToConsumerForDateTimeWithDescription("ana@example.com", "100000.00", "2026-07-04T12:00:00Z", &godog.DocString{Content: "Plumbing service"}); err != nil {
		return err
	}
	return s.systemRegistersServiceProposal()
}
func (s *testSuite) noticeDepositReady() error {
	if err := s.noticeProposalReady(); err != nil {
		return err
	}
	if err := s.noticeProposal(); err != nil {
		return err
	}
	s.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
	return s.startCheckoutForPreparedProposal()
}
func (s *testSuite) noticeScheduledOrder(after time.Duration) error {
	if err := s.prepareLinkedMercadoPagoAccount("mp-juan", "juan.plomero@example.com"); err != nil {
		return err
	}
	return s.createScheduledWorkOrderFixture("juan.plomero@example.com", "ana@example.com", 10000000, s.clock.Now().Add(after), "Plumbing service")
}
func (s *testSuite) noticeAtScheduledTime() error {
	o, err := s.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	return s.systemDateTimeIs(o.ScheduledOn().UTC().Format(time.RFC3339))
}
func (s *testSuite) noticeCompletion() error {
	s.pushCapture.reset()
	return s.providerReportedValidCompletion("juan.plomero@example.com")
}
func (s *testSuite) noticeCompletedOrder() error {
	if err := s.noticeScheduledOrder(48 * time.Hour); err != nil {
		return err
	}
	return s.noticeCompletion()
}
func (s *testSuite) noticeBalanceReady() error {
	if err := s.noticeCompletedOrder(); err != nil {
		return err
	}
	return s.consumerStartedServiceBalanceCheckout("ana@example.com")
}
func (s *testSuite) noticeFinalPayment() error {
	s.pushCapture.reset()
	return s.processApprovedPayment()
}
func (s *testSuite) noticeMessage(author, kind string) error {
	s.pushCapture.reset()
	s.currentAuth0ID = auth0IDForConsumerEmail("ana@example.com")
	if author == "Juan" {
		s.currentAuth0ID = auth0IDForProviderEmail("juan.plomero@example.com")
	}
	request := sendMessageRequest{Content: "Hello"}
	switch kind {
	case "con varias imágenes", "private":
		if err := s.uploadAndConfirmTwoMessageImages("home-one.jpg", "home-two.jpg"); err != nil {
			return err
		}
		for _, name := range []string{"home-one.jpg", "home-two.jpg"} {
			request.ImageFileIDs = append(request.ImageFileIDs, s.messageImagesByName[name].FileID)
		}
		if kind == "private" {
			request.Content = "Ana Gomez, street address 123"
		}
	case "de audio":
		if err := s.uploadAndConfirmMessageAudio("voice.webm", "18"); err != nil {
			return err
		}
		request.Content = ""
		request.AudioFileID = s.messageAudiosByName["voice.webm"].FileID
	case "con video":
		if err := s.uploadAndConfirmMessageVideo("service.mp4", "18"); err != nil {
			return err
		}
		request.VideoFileID = s.messageVideosByName["service.mp4"].FileID
	}
	if err := s.requestSendMessageToPreparedConversation(request); err != nil {
		return err
	}
	if kind == "de audio" {
		return s.systemRegistersAudioMessage("voice.webm")
	}
	return s.systemRegistersMessageInConversation()
}
func (s *testSuite) assertNotice(names []string, kind, resource string, resourceID int) error {
	requests := s.pushCapture.snapshot()
	if len(requests) != len(names) {
		return fmt.Errorf("expected %d pushes, got %d", len(names), len(requests))
	}
	seen := map[string]bool{}
	for _, r := range requests {
		name := ""
		for _, candidate := range names {
			if r.Message.Token == "token-"+candidate {
				name = candidate
			}
		}
		if name == "" || seen[name] {
			return fmt.Errorf("unexpected push recipient")
		}
		seen[name] = true
		id, err := s.userRepository.FindIDByEmail(noticeEmail(name))
		if err != nil {
			return err
		}
		d := r.Message.Data
		if d["type"] != kind || d["resource_type"] != resource || d["destination"] != resource || d["resource_id"] != strconv.Itoa(resourceID) || d["recipient_user_id"] != strconv.Itoa(id) || d["binding_id"] != "binding-"+name || d["installation_id"] != "phone-"+name || d["version"] != "1" || d["event_id"] == "" || d["title"] == "" || d["body"] == "" {
			return fmt.Errorf("push contract mismatch for %s", kind)
		}
		expires, err := time.Parse(time.RFC3339, d["expires_at"])
		if err != nil || !expires.After(s.clock.Now()) {
			return fmt.Errorf("invalid push expiry")
		}
		if len(r.Message.Notification) != 0 || r.Message.Android["priority"] != "high" {
			return fmt.Errorf("expected high priority data-only push")
		}
	}
	return nil
}
func (s *testSuite) assertOrderNotice(name, kind string) error {
	o, err := s.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	return s.assertNotice([]string{name}, kind, "work_order", o.ID())
}
func (s *testSuite) assertNoticeReminder() error {
	o, err := s.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if err := s.assertNotice([]string{"Ana", "Juan"}, "work_order_close_to_scheduled_time", "work_order", o.ID()); err != nil {
		return err
	}
	for _, r := range s.pushCapture.snapshot() {
		if r.Message.Data["expires_at"] != o.ScheduledOn().UTC().Format(time.RFC3339) || r.Message.Android["ttl"] != "82800s" {
			return fmt.Errorf("reminder must expire at scheduled time")
		}
	}
	return nil
}
func (s *testSuite) assertNoFinalNotice() error {
	for _, r := range s.pushCapture.snapshot() {
		if r.Message.Data["type"] == "work_order_final_payment_approved" {
			return fmt.Errorf("unexpected final payment push")
		}
	}
	return nil
}
func (s *testSuite) noticeLocale(preference string) error {
	locale := ""
	if preference == "en inglés" {
		locale = "en"
	}
	if preference == "en español" {
		locale = "es"
	}
	return s.noticePhone("Ana", locale)
}
func (s *testSuite) assertNoticeLocale(language string) error {
	if err := s.assertNotice([]string{"Ana"}, "service_proposal_received", "service_proposal", s.lastServiceProposalID); err != nil {
		return err
	}
	title, body := "Nueva propuesta", "Recibiste una propuesta de servicio."
	if language == "inglés" {
		title, body = "New proposal", "You received a service proposal."
	}
	d := s.pushCapture.snapshot()[0].Message.Data
	if d["title"] != title || d["body"] != body {
		return fmt.Errorf("incorrect locale template")
	}
	return nil
}
