package steps_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/cucumber/godog"
)

type categoryImpactResponse struct {
	Category   administeredCategoryResponse `json:"category"`
	ObservedAt time.Time                    `json:"observed_at"`
	Counts     struct {
		AssignedProviders               int `json:"assigned_providers"`
		PendingRequests                 int `json:"pending_requests"`
		AcceptedRequestsWithoutProposal int `json:"accepted_requests_without_proposal"`
		PendingProposals                int `json:"pending_proposals"`
		ScheduledOrders                 int `json:"scheduled_orders"`
		AwaitingPaymentOrders           int `json:"awaiting_payment_orders"`
	} `json:"counts"`
	HasOngoingOrders              bool `json:"has_ongoing_orders"`
	RequiresConfirmation          bool `json:"requires_confirmation"`
	ExistingOperationsCanContinue bool `json:"existing_operations_can_continue"`
}

func registerCategoryImpactSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que no hay prestadores ni operaciones asociadas al rubro "([^"]*)"$`, func(name string) error {
		id, err := s.categoryIDFor(name)
		if err != nil {
			return err
		}
		providers, err := s.userRepository.FindProvidersByCategoryID(id)
		if err != nil {
			return err
		}
		if len(providers) != 0 {
			return fmt.Errorf("unexpected providers")
		}
		return nil
	})
	sc.Step(`^consulto el impacto del rubro "([^"]*)"$`, s.categoryImpactRequest)
	sc.Step(`^el impacto informa el rubro (habilitado|deshabilitado) y su versión(?: a la fecha "([^"]*)")?$`, s.categoryImpactState)
	sc.Step(`^el impacto cuenta una asignación de prestador y una orden programada$`, func() error {
		response, err := s.categoryImpactBody()
		if err != nil {
			return err
		}
		if response.Counts.AssignedProviders != 1 || response.Counts.ScheduledOrders != 1 {
			return fmt.Errorf("impact counts mismatch")
		}
		return nil
	})
	sc.Step(`^el impacto informa cero solicitudes pendientes, aceptadas sin propuesta, propuestas pendientes y órdenes awaiting_payment$`, func() error {
		response, err := s.categoryImpactBody()
		if err != nil {
			return err
		}
		if response.Counts.PendingRequests != 0 || response.Counts.AcceptedRequestsWithoutProposal != 0 || response.Counts.PendingProposals != 0 || response.Counts.AwaitingPaymentOrders != 0 {
			return fmt.Errorf("unexpected impact activity")
		}
		return nil
	})
	sc.Step(`^los seis conteos de impacto son cero$`, func() error {
		response, err := s.categoryImpactBody()
		if err != nil {
			return err
		}
		if response.Counts.AssignedProviders != 0 || response.Counts.PendingRequests != 0 || response.Counts.AcceptedRequestsWithoutProposal != 0 || response.Counts.PendingProposals != 0 || response.Counts.ScheduledOrders != 0 || response.Counts.AwaitingPaymentOrders != 0 {
			return fmt.Errorf("nonempty impact counts")
		}
		return nil
	})
	sc.Step(`^el impacto indica que hay órdenes en curso y se requiere confirmación adicional para deshabilitar el rubro$`, func() error {
		response, err := s.categoryImpactBody()
		if err != nil {
			return err
		}
		if !response.HasOngoingOrders || !response.RequiresConfirmation {
			return fmt.Errorf("missing ongoing orders confirmation")
		}
		return nil
	})
	sc.Step(`^el impacto informa que las operaciones existentes, incluidas las negociaciones previas, pueden continuar aunque se deshabilite el rubro$`, func() error {
		response, err := s.categoryImpactBody()
		if err != nil {
			return err
		}
		if !response.ExistingOperationsCanContinue {
			return fmt.Errorf("existing operations continuation missing")
		}
		return nil
	})
	sc.Step(`^la consulta no registra un evento de auditoría(?: ni modifica el rubro)?$`, func() error {
		if err := s.categoryNoEditAudit(); err != nil {
			return err
		}
		return s.allCategoriesUnchanged()
	})
	sc.Step(`^la orden de trabajo conserva su estado programado(?: y la consulta no modifica las operaciones)?$`, s.categoryScheduledOrderUnchanged)
}
func (s *testSuite) categoryImpactRequest(name string) error {
	id, err := s.categoryIDFor(name)
	if err != nil {
		return err
	}
	if err = s.rememberCategoryOriginal(id); err != nil {
		return err
	}
	if err = s.categoryAuditBaseline(); err != nil {
		return err
	}
	s.categoryAdministration.operations = map[string]*operationDetailAuditSnapshot{}
	for label := range s.operationInbox.requests {
		snapshot, err := s.captureOperationDetailAuditSnapshot(label)
		if err != nil {
			return err
		}
		s.categoryAdministration.operations[label] = snapshot
	}
	if s.lastServiceProposalID != 0 {
		order, err := s.persistedWorkOrderForLastServiceProposal()
		if err != nil {
			return err
		}
		s.categoryAdministration.order = order
	}
	s.categoryAuditCapture.resetAttempt()
	s.lastCategoryAuditEventIDs = nil
	if s.invalidSession {
		s.adminRequest.omitBearer = true
	}
	return s.sendAdminGet("/admin/categories/"+strconv.Itoa(id)+"/impact", nil, "")
}
func (s *testSuite) categoryImpactBody() (*categoryImpactResponse, error) {
	if err := s.lastResponseShouldHaveStatusCode(200); err != nil {
		return nil, err
	}
	var response categoryImpactResponse
	if err := json.Unmarshal(s.lastBody, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
func (s *testSuite) categoryImpactState(state, instant string) error {
	response, err := s.categoryImpactBody()
	if err != nil {
		return err
	}
	original := s.categoryAdministration.original[s.categoryAdministration.selected]
	if response.Category.ID != original.ID || response.Category.Name != original.Name || response.Category.Version != original.Version || response.Category.Enabled != (state == "habilitado") {
		return fmt.Errorf("impact category mismatch")
	}
	if instant != "" {
		at, err := time.Parse(time.RFC3339, instant)
		if err != nil {
			return err
		}
		if !response.ObservedAt.Equal(at) {
			return fmt.Errorf("impact observed_at mismatch")
		}
	}
	return nil
}
func (s *testSuite) categoryScheduledOrderUnchanged() error {
	order, err := s.persistedWorkOrderForLastServiceProposal()
	if err != nil {
		return err
	}
	if before := s.categoryAdministration.order; before != nil && !reflect.DeepEqual(before, order) {
		return fmt.Errorf("order changed during impact read")
	}
	for label, before := range s.categoryAdministration.operations {
		after, err := s.captureOperationDetailAuditSnapshot(label)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(before.request, after.request) || !reflect.DeepEqual(before.conversation, after.conversation) || !reflect.DeepEqual(before.proposals, after.proposals) {
			return fmt.Errorf("operation changed during impact read")
		}
	}
	if order.Status() != "scheduled" {
		return fmt.Errorf("scheduled order changed")
	}
	return nil
}
