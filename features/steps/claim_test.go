package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	mediaadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/media"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type claimFixture struct {
	claim    *claim.Claim
	input    claim.Submission
	owner    string
	snapshot *claim.Claim
}
type claimState struct {
	claims    map[string]*claimFixture
	images    map[string]string
	paymentID string
	lastInput claim.Submission
	lastID    int
	lastKey   string
	selected  string
}

func (s *testSuite) claimMaps() {
	if s.claims.claims == nil {
		s.claims.claims = map[string]*claimFixture{}
		s.claims.images = map[string]string{}
	}
}
func registerClaimSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que existen el consumidor registrado "([^"]*)" y el prestador registrado "([^"]*)"$`, s.claimAccounts)
	sc.Step(`^que ambos participan en la solicitud de trabajo "([^"]*)"$`, s.claimRequest)
	sc.Step(`^que la solicitud "([^"]*)" no tiene propuesta, orden ni intento de pago$`, s.claimNoProposal)
	sc.Step(`^presento un reclamo por incumplimiento sobre la solicitud "([^"]*)" con la clave "([^"]*)" y el testimonio "([^"]*)"$`, func(label, key, text string) error {
		return s.submitClaimReference("job_request", label, key, "non_compliance", text, nil)
	})
	sc.Step(`^el sistema responde con estado 201 y la ubicación del reclamo creado$`, s.claimCreated)
	sc.Step(`^el acuse identifica la operación "([^"]*)" seguida del ID persistido de "([^"]*)", la referencia "([^"]*)" con el ID persistido de "([^"]*)" y el estado "([^"]*)"$`, s.claimAckMatches)
	sc.Step(`^el expediente registra como reclamante "([^"]*)" y como tipo de parte "([^"]*)"$`, s.claimOwnerMatches)
	sc.Step(`^el expediente conserva el testimonio presentado y no inventa una orden ni evidencia adjunta$`, s.claimOriginal)
	sc.Step(`^que existen dos propuestas de servicio de la conversación de "([^"]*)": "([^"]*)" es la primera y "([^"]*)" es posterior$`, s.claimTwoProposals)
	sc.Step(`^presento un reclamo por mala calidad sobre la propuesta "([^"]*)" con la clave "([^"]*)" y el testimonio "([^"]*)"$`, func(label, key, text string) error {
		return s.submitClaimReference("service_proposal", label, key, "poor_quality", text, nil)
	})
	sc.Step(`^el expediente queda vinculado a "([^"]*)" seguido del ID persistido de "([^"]*)" y conserva la referencia "([^"]*)" con el ID persistido de "([^"]*)"$`, func(prefix, label, kind, reference string) error {
		return s.claimAckMatches(prefix, label, kind, reference, "open")
	})
	sc.Step(`^el expediente no se atribuye a "([^"]*)" seguido del ID persistido de "([^"]*)" por compartir conversación o participantes$`, s.claimNotOperation)
	sc.Step(`^que "([^"]*)" ya presentó el reclamo "([^"]*)" sobre "([^"]*)" usando la solicitud como referencia y el expediente sigue "([^"]*)"$`, func(email, label, request, status string) error {
		return s.fixtureClaim(email, label, request, uuid.NewString(), "non_compliance", "Original testimony", nil, claim.Status(status), s.clock.Now())
	})
	sc.Step(`^que "([^"]*)" es la primera propuesta de la conversación de "([^"]*)"$`, s.claimFirstProposal)
	sc.Step(`^presento otro reclamo sobre la propuesta "([^"]*)" con una clave nueva y un testimonio diferente$`, func(label string) error {
		return s.submitClaimReference("service_proposal", label, uuid.NewString(), "damage", "Different testimony", nil)
	})
	sc.Step(`^el sistema responde con estado 409 sin revelar información de reclamos ajenos$`, func() error { return s.claimError(409) })
	sc.Step(`^el reclamo "([^"]*)" conserva su testimonio, estado y evidencia sin duplicar actuaciones$`, s.claimUnchanged)
	sc.Step(`^que "([^"]*)" presentó el reclamo "([^"]*)" sobre "([^"]*)" con la clave "([^"]*)", motivo "([^"]*)" y testimonio "([^"]*)"$`, func(email, label, request, key, reason, text string) error {
		return s.fixtureClaim(email, label, request, key, reason, text, nil, claim.StatusOpen, s.clock.Now())
	})
	sc.Step(`^presento el mismo reclamo sobre la solicitud "([^"]*)" con la misma clave "([^"]*)" y el mismo contenido$`, func(label, key string) error {
		f := s.claims.claims["C1"]
		return s.submitClaimReference("job_request", label, key, string(f.input.Reason), f.input.Description, f.input.ImageFileIDs)
	})
	sc.Step(`^el sistema responde con estado 201 y crea un expediente propio distinto de "([^"]*)" para "([^"]*)"$`, s.claimNewOwner)
	sc.Step(`^la respuesta no identifica ni devuelve el expediente "([^"]*)" de "([^"]*)"$`, s.claimResponseNotOther)
	sc.Step(`^que "([^"]*)" cargó y confirmó la imagen JPEG válida "([^"]*)" y la imagen PNG válida "([^"]*)" con finalidad "claim_evidence_image"$`, s.claimJPEGPNG)
	sc.Step(`^que ambas imágenes son privadas y de hasta 5 MiB cada una$`, s.claimImagesPrivate)
	sc.Step(`^presento un reclamo sobre la solicitud "([^"]*)" con esas imágenes y el testimonio "([^"]*)"$`, func(label, text string) error {
		return s.submitClaimReference("job_request", label, uuid.NewString(), "non_compliance", text, []string{s.claims.images["IMG1"], s.claims.images["IMG2"]})
	})
	sc.Step(`^el expediente conserva los IDs estables "([^"]*)" y "([^"]*)" como evidencias vinculadas$`, s.claimEvidenceMatches)
	sc.Step(`^la evidencia no se representa mediante una URL firmada ni se expone públicamente$`, s.claimNoURLs)
	sc.Step(`^que la solicitud "S2" pertenece a otra persona, "S404" no existe y "P1" es una propuesta de "S1" con el intento de pago "PI1"$`, s.claimUnauthorizedReferences)
	sc.Step(`^que el prestador no está autorizado a consultar "PI1" por las reglas del módulo de pagos$`, s.claimPrivatePayment)
	sc.Step(`^que el request es válido salvo por la autorización o existencia de la referencia$`, s.claimValidRequest)
	sc.Step(`^intento iniciar un reclamo con la referencia "(.*)"$`, s.claimUnauthorizedSubmit)
	sc.Step(`^el sistema responde con estado 404 sin crear un expediente ni revelar datos de la referencia$`, func() error { return s.claimErrorNoCreation(404) })
	sc.Step(`^que la credencial de autenticación es "([^"]*)"$`, s.claimCredential)
	sc.Step(`^intento presentar un reclamo sobre la solicitud "([^"]*)"$`, func(label string) error {
		return s.submitClaimReference("job_request", label, uuid.NewString(), "damage", "Original testimony", nil)
	})
	sc.Step(`^el sistema responde con estado (\d+) sin crear un expediente$`, s.claimErrorNoCreation)
	sc.Step(`^que "([^"]*)" presentó el reclamo "([^"]*)" con la clave "([^"]*)" y el contenido original está asociado a esa clave$`, s.claimFixtureKey)
	sc.Step(`^que el estado actual del reclamo "([^"]*)" es "([^"]*)"$`, s.claimStateFixture)
	sc.Step(`^repito el ingreso de "([^"]*)" con la misma clave y el mismo contenido$`, s.claimReplay)
	sc.Step(`^el sistema responde con estado 200 y el mismo identificador "([^"]*)" con su estado actual "([^"]*)"$`, s.claimReplayResponse)
	sc.Step(`^el expediente, sus actuaciones y sus vínculos de evidencia no se duplican$`, s.claimNoDuplicates)
	sc.Step(`^que "([^"]*)" presentó el reclamo "([^"]*)" con la clave "([^"]*)"$`, s.claimFixtureKey)
	sc.Step(`^intento presentar contenido distinto usando la clave ya asociada a "([^"]*)"$`, s.claimDifferentContent)
	sc.Step(`^el sistema responde con estado 409 sin crear ni modificar un expediente$`, s.claimConflictUnchanged)
	sc.Step(`^que "([^"]*)" tiene el reclamo finalizado "([^"]*)" con su dictamen formal persistido por el contrato compartido con US-68$`, s.claimFinalFixture)
	sc.Step(`^presento un nuevo reclamo sobre la misma operación con una clave nueva$`, func() error {
		return s.submitClaimReference("job_request", "S1", uuid.NewString(), "damage", "New testimony", nil)
	})
	sc.Step(`^el sistema responde con estado 201 y crea un expediente distinto "([^"]*)" en estado "([^"]*)"$`, s.claimCreatedDistinct)
	sc.Step(`^"([^"]*)" conserva su identidad, estado final y dictamen sin reapertura ni modificación$`, s.claimUnchanged)
	sc.Step(`^que "([^"]*)" tiene los reclamos abiertos "([^"]*)" y "([^"]*)" creados ambos el "([^"]*)", con el ID persistido de "([^"]*)" mayor que el de "([^"]*)", y "([^"]*)" creado el "([^"]*)", en operaciones distintas$`, s.claimPageFixture)
	sc.Step(`^que "([^"]*)" tiene el reclamo resuelto "([^"]*)" creado el "([^"]*)"$`, s.claimResolvedFixture)
	sc.Step(`^que "([^"]*)" tiene un reclamo propio más reciente creado el "([^"]*)"$`, s.claimOtherRecent)
	sc.Step(`^consulto mis reclamos abiertos en la página (\d+) con límite 2$`, func(page int) error { return s.claimList("open", page, 2) })
	sc.Step(`^el sistema responde con estado 200 y la colección contiene exactamente "([^"]*)" en ese orden$`, s.claimListExact)
	sc.Step(`^la respuesta informa la página solicitada (\d+) y el límite 2, y no contiene reclamos de "([^"]*)" ni el reclamo resuelto "([^"]*)"$`, s.claimPageMetadata)
	sc.Step(`^que "([^"]*)" no tiene reclamos en estado "([^"]*)"$`, s.claimNoState)
	sc.Step(`^consulto mis reclamos con estado "([^"]*)", página (\d+) y límite (\d+)$`, s.claimList)
	sc.Step(`^el sistema responde con estado 200 y una colección vacía$`, func() error { return s.claimListExact("ninguno") })
	sc.Step(`^la respuesta no incluye reclamos de "([^"]*)"$`, s.claimListNotOwner)
	sc.Step(`^que el reclamo "([^"]*)" (.*)$`, s.claimExistence)
	sc.Step(`^consulto el detalle propio del reclamo "([^"]*)"$`, s.claimGet)
	sc.Step(`^el sistema responde con estado 404 sin incluir testimonio, operación ni evidencias$`, func() error { return s.claimError(404) })
	sc.Step(`^que "([^"]*)" presentó el reclamo "([^"]*)" y no existe resolución registrada en el contrato compartido con US-68$`, func(email, label string) error { return s.claimFixtureKey(email, label, uuid.NewString()) })
	sc.Step(`^el detalle informa la resolución como vacía$`, s.claimResolutionNull)
	sc.Step(`^que "([^"]*)" presentó el reclamo finalizado "([^"]*)" con una resolución formal persistida por el contrato compartido con US-68$`, s.claimFinalFixture)
	sc.Step(`^que el dictamen de "([^"]*)" tiene tipo semántico "a favor del consumidor", fundamentación "([^"]*)" y fecha "([^"]*)"$`, s.claimResolutionFixture)
	sc.Step(`^que el dictamen sugiere una compensación de (\d+) centavos ARS, sin afirmar que se ejecutó$`, s.claimCompensationFixture)
	sc.Step(`^que "([^"]*)" tiene un expediente separado "([^"]*)" con la evidencia privada "([^"]*)", no vinculada a "([^"]*)"$`, s.claimSeparateEvidence)
	sc.Step(`^el detalle presenta exactamente el tipo, fundamentación y fecha registrados para "([^"]*)"$`, s.claimResolutionMatches)
	sc.Step(`^informa exactamente la compensación sugerida de (\d+) centavos ARS, sin afirmar que se ejecutó$`, s.claimCompensationMatches)
	sc.Step(`^la respuesta no expone la identidad privada del operador, auditorías de acceso, evidencias ajenas ni transacciones financieras completas$`, s.claimProjectionPrivate)
	sc.Step(`^que "([^"]*)" tiene el reclamo "([^"]*)" con la imagen privada confirmada "([^"]*)" vinculada$`, s.claimLinkedImage)
	sc.Step(`^que "([^"]*)" tiene el reclamo "([^"]*)" y "([^"]*)" tiene el reclamo "([^"]*)" en la misma operación$`, s.claimTwoOwners)
	sc.Step(`^que la imagen privada confirmada "([^"]*)" está vinculada a "([^"]*)" y no a "([^"]*)"$`, s.claimImageLinkedOnlyTo)
	sc.Step(`^la respuesta no entrega una URL temporal$`, s.claimResponseHasNoURL)
	sc.Step(`^el detalle no incluye la imagen "([^"]*)" ni una URL temporal para ella$`, s.claimDetailExcludesImage)
	sc.Step(`^la respuesta no expone claves de almacenamiento ni credenciales$`, s.claimProjectionPrivate)
}
func (s *testSuite) claimAccounts(consumer, provider string) error {
	s.claimMaps()
	if err := s.thereIsRegisteredConsumerWithEmailNameAndSurname(consumer, "Ana", "Consumer"); err != nil {
		return err
	}
	if err := s.thereAreCategories("Claims", "Claims other"); err != nil {
		return err
	}
	return s.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(provider, "Juan", "Provider", "Claims")
}
func (s *testSuite) claimRequest(label string) error {
	s.operationInbox.ensureMaps()
	return s.createInboxJobRequest(label, "ana@example.com", "juan@example.com", time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC), "pending")
}
func (s *testSuite) claimNoProposal(label string) error {
	actor, err := repositories.NewClaimUserFinder(s.database).FindClaimantByAuthID(context.Background(), auth0IDForConsumerEmail("ana@example.com"))
	if err != nil {
		return err
	}
	proposals, err := repositories.NewServiceProposalRepository(s.database).FindByUserID(context.Background(), actor.ID)
	if err != nil {
		return err
	}
	if len(proposals) != 0 {
		return fmt.Errorf("unexpected proposals")
	}
	return nil
}
func (s *testSuite) claimReference(kind, label string) (claim.Reference, error) {
	id := ""
	switch kind {
	case "job_request":
		request, exists := s.operationInbox.requests[label]
		if !exists && label != "S404" {
			return claim.Reference{}, fmt.Errorf("unknown request %s", label)
		}
		id = strconv.Itoa(request.id)
		if label == "S404" {
			id = "2147483647"
		}
	case "service_proposal":
		id = strconv.Itoa(s.operationInbox.proposals[label].id)
	case "payment_intent":
		id = s.claims.paymentID
	}
	return claim.NewReference(claim.ReferenceKind(kind), id)
}
func (s *testSuite) claimAuth(email string) string {
	if email == "juan@example.com" {
		return auth0IDForProviderEmail(email)
	}
	return auth0IDForConsumerEmail(email)
}
func (s *testSuite) claimRepo() *repositories.ClaimRepository {
	return repositories.NewClaimRepository(s.database)
}
func (s *testSuite) fixtureClaim(email, label, request, key, reason, text string, images []string, status claim.Status, now time.Time) error {
	s.claimMaps()
	ref, err := s.claimReference("job_request", request)
	if err != nil {
		return err
	}
	input, err := (claim.Submission{Reference: ref, Reason: claim.Reason(reason), Description: text, ImageFileIDs: images}).Normalize()
	if err != nil {
		return err
	}
	actor, err := repositories.NewClaimUserFinder(s.database).FindClaimantByAuthID(context.Background(), s.claimAuth(email))
	if err != nil {
		return err
	}
	operation, err := repositories.NewClaimOperationReferenceResolver(s.database).ResolveClaimOperationReference(context.Background(), *actor, ref)
	if err != nil {
		return err
	}
	found, err := claim.New(*actor, *operation, key, input, now)
	if err != nil {
		return err
	}
	if err := s.claimRepo().Save(context.Background(), found); err != nil {
		return err
	}
	s.claims.claims[label] = &claimFixture{claim: found, input: input, owner: email}
	if status != claim.StatusOpen {
		if err := s.claimStateFixture(label, string(status)); err != nil {
			return err
		}
	}
	return s.claimSnapshot(label)
}
func (s *testSuite) claimSnapshot(label string) error {
	f := s.claims.claims[label]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	f.claim = found
	f.snapshot = found
	return nil
}
func (s *testSuite) claimFixtureKey(email, label, key string) error {
	return s.fixtureClaim(email, label, "S1", key, "non_compliance", "Original testimony", nil, claim.StatusOpen, s.clock.Now())
}
func (s *testSuite) claimStateFixture(label, status string) error {
	f := s.claims.claims[label]
	var resolution *claim.Resolution
	if status == "resolved" || status == "dismissed" {
		kind := claim.ResolutionTypeConsumerFavor
		if status == "dismissed" {
			kind = claim.ResolutionTypeWithoutMerit
		}
		resolution = &claim.Resolution{Type: kind, Reasoning: "Formal reasoning", ResolvedOn: f.claim.CreatedOn.Add(time.Hour)}
	}
	if err := (testsupport.ClaimLifecycleFixture{DB: s.database}).SetState(context.Background(), f.claim.ID, claim.Status(status), resolution); err != nil {
		return err
	}
	return s.claimSnapshot(label)
}
func (s *testSuite) claimFinalFixture(email, label string) error {
	if err := s.fixtureClaim(email, label, "S1", uuid.NewString(), "non_compliance", "Original testimony", nil, claim.StatusOpen, time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)); err != nil {
		return err
	}
	resolution := &claim.Resolution{Type: claim.ResolutionTypeConsumerFavor, Reasoning: "Se verificó el incumplimiento acordado", ResolvedOn: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), SuggestedCompensation: &claim.SuggestedCompensation{AmountMinor: 1500, Currency: "ARS", Unit: "minor"}}
	if err := (testsupport.ClaimLifecycleFixture{DB: s.database}).SetState(context.Background(), s.claims.claims[label].claim.ID, claim.StatusResolved, resolution); err != nil {
		return err
	}
	return s.claimSnapshot(label)
}

func (s *testSuite) submitClaimReference(kind, label, key, reason, text string, images []string) error {
	ref, err := s.claimReference(kind, label)
	if err != nil {
		return err
	}
	input, err := (claim.Submission{Reference: ref, Reason: claim.Reason(reason), Description: text, ImageFileIDs: images}).Normalize()
	if err != nil {
		return err
	}
	return s.claimSubmit(key, input)
}
func (s *testSuite) claimSubmit(key string, input claim.Submission) error {
	var id any = input.Reference.ID()
	if input.Reference.Kind() != claim.ReferenceKindPaymentIntent {
		number, err := strconv.Atoi(input.Reference.ID())
		if err != nil {
			return err
		}
		id = number
	}
	s.claims.lastInput = input
	s.claims.lastKey = key
	if err := s.sendAuthenticatedJSON(http.MethodPost, "/claims", key, map[string]any{"reference": map[string]any{string(input.Reference.Kind()) + "_id": id}, "reason": input.Reason, "description": input.Description, "image_file_ids": input.ImageFileIDs}); err != nil {
		return err
	}
	if s.lastStatus == 200 || s.lastStatus == 201 {
		var response struct{ ID int }
		if err := json.Unmarshal(s.lastBody, &response); err != nil {
			return err
		}
		s.claims.lastID = response.ID
	}
	return nil
}
func (s *testSuite) claimResponse() (map[string]json.RawMessage, error) {
	var response map[string]json.RawMessage
	err := json.Unmarshal(s.lastBody, &response)
	return response, err
}
func (s *testSuite) claimCreated() error {
	if s.lastStatus != 201 || s.lastLocation != "/claims/"+strconv.Itoa(s.claims.lastID) {
		return fmt.Errorf("expected claim creation with location, status %d body %s", s.lastStatus, s.lastBody)
	}
	return nil
}
func (s *testSuite) claimAckMatches(prefix, label, kind, referenceLabel, status string) error {
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	resource := s.operationInbox.requests[label].id
	if prefix == "sp-" {
		resource = s.operationInbox.proposals[label].id
	}
	var operation string
	var actualStatus string
	var ref map[string]any
	if err := json.Unmarshal(response["operation_id"], &operation); err != nil {
		return err
	}
	if err := json.Unmarshal(response["status"], &actualStatus); err != nil {
		return err
	}
	if err := json.Unmarshal(response["reference"], &ref); err != nil {
		return err
	}
	refID := s.operationInbox.requests[referenceLabel].id
	if kind == "service_proposal_id" {
		refID = s.operationInbox.proposals[referenceLabel].id
	}
	if operation != prefix+strconv.Itoa(resource) || actualStatus != status || len(ref) != 1 || ref[kind] != float64(refID) {
		return fmt.Errorf("unexpected acknowledgment %s", s.lastBody)
	}
	return nil
}
func (s *testSuite) claimPersisted() (*claim.Claim, error) {
	actor, err := repositories.NewClaimUserFinder(s.database).FindClaimantByAuthID(context.Background(), s.currentAuth0ID)
	if err != nil {
		return nil, err
	}
	return s.claimRepo().FindOwnedByID(context.Background(), actor.ID, s.claims.lastID)
}
func (s *testSuite) claimOwnerMatches(email, party string) error {
	found, err := s.claimPersisted()
	if err != nil {
		return err
	}
	id, err := s.userRepository.FindIDByEmail(email)
	if err != nil {
		return err
	}
	if found == nil || found.ClaimantID != id || string(found.ClaimantParty) != party {
		return fmt.Errorf("unexpected claimant")
	}
	return nil
}
func (s *testSuite) claimOriginal() error {
	found, err := s.claimPersisted()
	if err != nil {
		return err
	}
	if found.Description != s.claims.lastInput.Description || len(found.ImageFileIDs) != 0 || found.Reference.Kind() != claim.ReferenceKindJobRequest || len(found.Actions) != 1 {
		return fmt.Errorf("unexpected persisted submission")
	}
	return nil
}
func (s *testSuite) claimTwoProposals(request, first, second string) error {
	if err := s.claimFirstProposal(first, request); err != nil {
		return err
	}
	return s.createInboxServiceProposal(second, request, s.clock.Now().Add(time.Minute), time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC), 60, "pending")
}
func (s *testSuite) claimFirstProposal(label, request string) error {
	if err := s.acceptInboxJobRequest(request); err != nil {
		return err
	}
	return s.createInboxServiceProposal(label, request, s.clock.Now(), time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC), 60, "pending")
}
func (s *testSuite) claimNotOperation(prefix, label string) error {
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	if string(response["operation_id"]) == strconv.Quote(prefix+strconv.Itoa(s.operationInbox.requests[label].id)) {
		return fmt.Errorf("wrong operation")
	}
	return nil
}
func (s *testSuite) claimError(status int) error {
	if s.lastStatus != status {
		return fmt.Errorf("expected %d got %d body %s", status, s.lastStatus, s.lastBody)
	}
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	if _, exists := response["error"]; !exists {
		return fmt.Errorf("missing error")
	}
	if status == http.StatusUnauthorized {
		if len(response) != 2 || string(response["error"]) != strconv.Quote("invalid_token") || string(response["message"]) != strconv.Quote("Failed to validate JWT.") {
			return fmt.Errorf("unexpected authentication error contract %s", s.lastBody)
		}
		return nil
	}
	if len(response) != 1 {
		return fmt.Errorf("error leaks fields %s", s.lastBody)
	}
	return nil
}
func (s *testSuite) claimErrorNoCreation(status int) error {
	if err := s.claimError(status); err != nil {
		return err
	}
	for _, email := range []string{"ana@example.com", "juan@example.com"} {
		actor, err := repositories.NewClaimUserFinder(s.database).FindClaimantByAuthID(context.Background(), s.claimAuth(email))
		if err != nil {
			return err
		}
		page, err := s.claimRepo().FindOwnedPage(context.Background(), actor.ID, claim.ListCriteria{Page: 1, Limit: 100})
		if err != nil {
			return err
		}
		if page.Total != 0 {
			return fmt.Errorf("unexpected created claim")
		}
	}
	return nil
}
func (s *testSuite) claimUnchanged(label string) error {
	f := s.claims.claims[label]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(found, f.snapshot) {
		return fmt.Errorf("claim %s changed", label)
	}
	return nil
}
func (s *testSuite) claimNewOwner(label, email string) error {
	if err := s.claimCreated(); err != nil {
		return err
	}
	if s.claims.lastID == s.claims.claims[label].claim.ID {
		return fmt.Errorf("same claim ID")
	}
	return s.claimOwnerMatches(email, "provider")
}
func (s *testSuite) claimResponseNotOther(label, email string) error {
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	var id int
	if err := json.Unmarshal(response["id"], &id); err != nil {
		return err
	}
	if id == s.claims.claims[label].claim.ID {
		return fmt.Errorf("returned other claimant's ID")
	}
	return s.claimProjectionPrivate()
}
func (s *testSuite) claimFileService() (*filedomain.Service, error) {
	components, err := storage.NewComponentsFromEnv()
	if err != nil {
		return nil, err
	}
	return filedomain.NewService(s.fileRepository, components.Storage, components.PublicBucket, components.PrivateBucket, s.clock, mediaadapter.NewWebMAudioParser(), mediaadapter.NewMP4VideoParser()), nil
}
func (s *testSuite) claimUploadImage(email, label, mime string) error {
	svc, err := s.claimFileService()
	if err != nil {
		return err
	}
	var data []byte
	switch mime {
	case "image/jpeg":
		var encoded bytes.Buffer
		if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
			return err
		}
		data = encoded.Bytes()
	default:
		data, err = testsupport.ClaimImagePNG()
		if err != nil {
			return err
		}
	}
	auth := s.claimAuth(email)
	target, err := svc.RequestUpload(context.Background(), filedomain.PresignRequest{AuthID: auth, OriginalName: label, MimeType: mime, SizeBytes: len(data), Purpose: filedomain.PurposeClaimEvidenceImage})
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPut, target.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	for key, value := range target.Headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("image upload returned %d", response.StatusCode)
	}
	_, err = svc.ConfirmUpload(context.Background(), filedomain.ConfirmRequest{AuthID: auth, FileID: target.FileID, Key: target.Key, MimeType: mime, SizeBytes: len(data)})
	if err != nil {
		return err
	}
	s.claimMaps()
	s.claims.images[label] = target.FileID
	return nil
}
func (s *testSuite) claimJPEGPNG(email, first, second string) error {
	if err := s.claimUploadImage(email, first, "image/jpeg"); err != nil {
		return err
	}
	return s.claimUploadImage(email, second, "image/png")
}
func (s *testSuite) claimImagesPrivate() error {
	for _, id := range s.claims.images {
		found, err := s.fileRepository.FindByID(context.Background(), id)
		if err != nil {
			return err
		}
		if found == nil || found.Status != filedomain.StatusConfirmed || found.Visibility != filedomain.VisibilityPrivate || found.Purpose != filedomain.PurposeClaimEvidenceImage || found.SizeBytes() > 5*1024*1024 {
			return fmt.Errorf("invalid private claim image")
		}
	}
	return nil
}
func (s *testSuite) claimEvidenceMatches(first, second string) error {
	found, err := s.claimPersisted()
	if err != nil {
		return err
	}
	expected := map[string]bool{s.claims.images[first]: true, s.claims.images[second]: true}
	if len(found.ImageFileIDs) != 2 {
		return fmt.Errorf("missing images")
	}
	for _, id := range found.ImageFileIDs {
		if !expected[id] {
			return fmt.Errorf("unexpected evidence ID")
		}
	}
	return nil
}
func (s *testSuite) claimNoURLs() error {
	found, err := s.claimPersisted()
	if err != nil {
		return err
	}
	for _, id := range found.ImageFileIDs {
		if _, err := uuid.Parse(id); err != nil {
			return err
		}
	}
	if strings.Contains(string(s.lastBody), "http") {
		return fmt.Errorf("submission contains URL")
	}
	return nil
}
func (s *testSuite) claimUnauthorizedReferences() error {
	if err := s.thereIsRegisteredConsumerWithEmailNameAndSurname("other@example.com", "Other", "Consumer"); err != nil {
		return err
	}
	otherProvider := "other-provider@example.com"
	if err := s.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(otherProvider, "Other", "Provider", "Claims"); err != nil {
		return err
	}
	if err := s.createInboxJobRequest("S2", "other@example.com", otherProvider, s.clock.Now(), "pending"); err != nil {
		return err
	}
	if err := s.claimFirstProposal("P1", "S1"); err != nil {
		return err
	}
	proposal, err := repositories.NewServiceProposalRepository(s.database).FindByID(context.Background(), s.operationInbox.proposals["P1"].id)
	if err != nil {
		return err
	}
	intent, err := payment.NewBookingDepositIntent(uuid.NewString(), proposal.ID, proposal.BookingTerms, s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.paymentIntentRepository.Save(context.Background(), intent); err != nil {
		return err
	}
	s.claims.paymentID = intent.ID
	return nil
}
func (s *testSuite) claimPrivatePayment() error {
	intent, err := s.paymentIntentRepository.FindByID(context.Background(), s.claims.paymentID)
	if err != nil {
		return err
	}
	if intent == nil || intent.ServiceProposalID != s.operationInbox.proposals["P1"].id {
		return fmt.Errorf("missing private payment fixture")
	}
	return nil
}
func (s *testSuite) claimValidRequest() error {
	ref, err := s.claimReference("job_request", "S1")
	if err != nil {
		return err
	}
	_, err = (claim.Submission{Reference: ref, Reason: claim.ReasonDamage, Description: "Original testimony"}).Normalize()
	return err
}
func (s *testSuite) claimUnauthorizedSubmit(reference string) error {
	kind, label := "job_request", "S2"
	if strings.Contains(reference, "S404") {
		label = "S404"
	}
	if strings.Contains(reference, "PI1") {
		kind = "payment_intent"
		label = "PI1"
	}
	return s.submitClaimReference(kind, label, uuid.NewString(), "damage", "Original testimony", nil)
}
func (s *testSuite) claimCredential(credential string) error {
	s.adminRequest.omitBearer = false
	s.adminRequest.invalidBearer = false
	switch credential {
	case "ausente":
		s.adminRequest.omitBearer = true
	case "JWT inválido":
		s.adminRequest.invalidBearer = true
	case "identidad sin cuenta local":
		s.currentAuth0ID = "auth0|claim-no-local-account"
	default:
		return fmt.Errorf("unknown credential")
	}
	return nil
}
func (s *testSuite) claimReplay(label string) error {
	f := s.claims.claims[label]
	s.claims.selected = label
	return s.claimSubmit(f.claim.SubmissionKey(), f.input)
}
func (s *testSuite) claimReplayResponse(label, status string) error {
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	if s.lastStatus != 200 || s.claims.lastID != s.claims.claims[label].claim.ID || string(response["status"]) != strconv.Quote(status) {
		return fmt.Errorf("unexpected replay %s", s.lastBody)
	}
	return nil
}
func (s *testSuite) claimNoDuplicates() error {
	label := s.claims.selected
	if err := s.claimUnchanged(label); err != nil {
		return err
	}
	f := s.claims.claims[label]
	page, err := s.claimRepo().FindOwnedPage(context.Background(), f.claim.ClaimantID, claim.ListCriteria{Page: 1, Limit: 100})
	if err != nil {
		return err
	}
	if page.Total != 1 {
		return fmt.Errorf("duplicated claim")
	}
	return nil
}
func (s *testSuite) claimDifferentContent(label string) error {
	f := s.claims.claims[label]
	input := f.input
	input.Description = "Different testimony"
	s.claims.selected = label
	return s.claimSubmit(f.claim.SubmissionKey(), input)
}
func (s *testSuite) claimConflictUnchanged() error {
	if err := s.claimError(409); err != nil {
		return err
	}
	return s.claimNoDuplicates()
}
func (s *testSuite) claimCreatedDistinct(label, status string) error {
	if err := s.claimCreated(); err != nil {
		return err
	}
	if s.claims.lastID == s.claims.claims["C1"].claim.ID {
		return fmt.Errorf("claim reopened")
	}
	found, err := s.claimPersisted()
	if err != nil {
		return err
	}
	if string(found.Status) != status {
		return fmt.Errorf("wrong new state")
	}
	s.claims.claims[label] = &claimFixture{claim: found, input: s.claims.lastInput, owner: "ana@example.com", snapshot: found}
	return nil
}

// claimDistinctRequest prepares a separate operation using the existing page-fixture pattern.
func (s *testSuite) claimDistinctRequest(request, consumer string, created time.Time) error {
	return s.withInboxFixtureClock(func() error {
		providerEmail := "provider-" + request + "@example.com"
		if err := s.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(providerEmail, "Fixture", request, "Claims"); err != nil {
			return err
		}
		return s.createInboxJobRequest(request, consumer, providerEmail, created, "pending")
	})
}
func (s *testSuite) claimPageFixture(email, first, second, instant, higher, lower, third, thirdInstant string) error {
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	last, err := time.Parse(time.RFC3339, thirdInstant)
	if err != nil {
		return err
	}
	for _, entry := range []struct {
		label string
		now   time.Time
	}{{first, now}, {second, now}, {third, last}} {
		request := "request-" + entry.label
		if err := s.claimDistinctRequest(request, email, entry.now); err != nil {
			return err
		}
		if err := s.fixtureClaim(email, entry.label, request, uuid.NewString(), "damage", "Original testimony", nil, claim.StatusOpen, entry.now); err != nil {
			return err
		}
	}
	if s.claims.claims[higher].claim.ID <= s.claims.claims[lower].claim.ID {
		return fmt.Errorf("fixture IDs out of order")
	}
	return nil
}
func (s *testSuite) claimResolvedFixture(email, label, instant string) error {
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	return s.fixtureClaim(email, label, "S1", uuid.NewString(), "damage", "Resolved testimony", nil, claim.StatusResolved, now)
}
func (s *testSuite) claimOtherRecent(email, instant string) error {
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	return s.fixtureClaim(email, "other-claim", "S1", uuid.NewString(), "damage", "Other testimony", nil, claim.StatusOpen, now)
}
func (s *testSuite) claimList(status string, page, limit int) error {
	return s.sendAuthenticatedJSON(http.MethodGet, fmt.Sprintf("/claims?status=%s&page=%d&limit=%d", url.QueryEscape(status), page, limit), "", nil)
}
func (s *testSuite) claimPage() (struct {
	Items              []struct{ ID int }
	Page, Limit, Total int
}, error) {
	var response struct {
		Items              []struct{ ID int }
		Page, Limit, Total int
	}
	err := json.Unmarshal(s.lastBody, &response)
	return response, err
}
func (s *testSuite) claimListExact(labels string) error {
	if s.lastStatus != 200 {
		return fmt.Errorf("list returned %d: %s", s.lastStatus, s.lastBody)
	}
	response, err := s.claimPage()
	if err != nil {
		return err
	}
	expected := []int{}
	if labels != "ninguno" {
		for _, label := range strings.Split(labels, ",") {
			f := s.claims.claims[strings.TrimSpace(label)]
			if f == nil {
				return fmt.Errorf("unknown claim label")
			}
			expected = append(expected, f.claim.ID)
		}
	}
	if len(response.Items) != len(expected) {
		return fmt.Errorf("unexpected list %s", s.lastBody)
	}
	for i, item := range response.Items {
		if item.ID != expected[i] {
			return fmt.Errorf("unexpected claim order")
		}
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	if labels == "ninguno" && string(raw["items"]) != "[]" {
		return fmt.Errorf("empty collection is not []")
	}
	return nil
}
func (s *testSuite) claimPageMetadata(page int, email, label string) error {
	response, err := s.claimPage()
	if err != nil {
		return err
	}
	if response.Page != page || response.Limit != 2 || response.Total != 3 {
		return fmt.Errorf("wrong pagination metadata")
	}
	for _, item := range response.Items {
		if item.ID == s.claims.claims[label].claim.ID {
			return fmt.Errorf("resolved claim in open page")
		}
	}
	return s.claimListNotOwner(email)
}
func (s *testSuite) claimListNotOwner(email string) error {
	response, err := s.claimPage()
	if err != nil {
		return err
	}
	for _, item := range response.Items {
		for _, fixture := range s.claims.claims {
			if fixture.owner == email && fixture.claim.ID == item.ID {
				return fmt.Errorf("list exposes another owner's claim")
			}
		}
	}
	return nil
}
func (s *testSuite) claimNoState(email, status string) error {
	actor, err := repositories.NewClaimUserFinder(s.database).FindClaimantByAuthID(context.Background(), s.claimAuth(email))
	if err != nil {
		return err
	}
	state := claim.Status(status)
	page, err := s.claimRepo().FindOwnedPage(context.Background(), actor.ID, claim.ListCriteria{Status: &state, Page: 1, Limit: 20})
	if err != nil {
		return err
	}
	if page.Total != 0 {
		return fmt.Errorf("unexpected state fixture")
	}
	return nil
}
func (s *testSuite) claimExistence(label, existence string) error {
	if existence == "no existe" {
		s.claims.claims[label] = &claimFixture{claim: &claim.Claim{ID: 2147483647}}
		return nil
	}
	return s.claimFixtureKey("ana@example.com", label, uuid.NewString())
}
func (s *testSuite) claimGet(label string) error {
	s.claims.selected = label
	return s.sendAuthenticatedJSON(http.MethodGet, "/claims/"+strconv.Itoa(s.claims.claims[label].claim.ID), "", nil)
}
func (s *testSuite) claimResolutionNull() error {
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	if s.lastStatus != 200 || string(response["resolution"]) != "null" {
		return fmt.Errorf("resolution must be present null: %s", s.lastBody)
	}
	return nil
}
func (s *testSuite) claimResolutionFixture(label, reasoning, instant string) error {
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	resolution := s.claims.claims[label].claim.Resolution
	if resolution == nil || resolution.Type != claim.ResolutionTypeConsumerFavor || resolution.Reasoning != reasoning || !resolution.ResolvedOn.Equal(now) {
		return fmt.Errorf("formal resolution fixture mismatch")
	}
	s.claims.selected = label
	return nil
}
func (s *testSuite) claimCompensationFixture(amount int) error {
	compensation := s.claims.claims[s.claims.selected].claim.Resolution.SuggestedCompensation
	if compensation == nil || compensation.AmountMinor != int64(amount) || compensation.Currency != "ARS" || compensation.Unit != "minor" {
		return fmt.Errorf("compensation fixture mismatch")
	}
	return nil
}
func (s *testSuite) claimSeparateEvidence(email, label, imageLabel, otherLabel string) error {
	if err := s.claimUploadImage(email, imageLabel, "image/png"); err != nil {
		return err
	}
	return s.fixtureClaim(email, label, "S1", uuid.NewString(), "damage", "Other private testimony", []string{s.claims.images[imageLabel]}, claim.StatusOpen, s.clock.Now())
}
func (s *testSuite) claimResolutionMatches(label string) error {
	var response struct {
		Resolution *struct {
			Type       claim.ResolutionType
			Reasoning  string
			ResolvedOn time.Time `json:"resolved_on"`
		}
	}
	if err := json.Unmarshal(s.lastBody, &response); err != nil {
		return err
	}
	expected := s.claims.claims[label].claim.Resolution
	if s.lastStatus != 200 || response.Resolution == nil || response.Resolution.Type != expected.Type || response.Resolution.Reasoning != expected.Reasoning || !response.Resolution.ResolvedOn.Equal(expected.ResolvedOn) {
		return fmt.Errorf("wrong formal resolution %s", s.lastBody)
	}
	return nil
}
func (s *testSuite) claimCompensationMatches(amount int) error {
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	var resolution struct {
		SuggestedCompensation *struct {
			AmountMinor int64 `json:"amount_minor"`
			Currency    string
			Unit        string
		} `json:"suggested_compensation"`
	}
	if err := json.Unmarshal(response["resolution"], &resolution); err != nil {
		return err
	}
	if resolution.SuggestedCompensation == nil || resolution.SuggestedCompensation.AmountMinor != int64(amount) || resolution.SuggestedCompensation.Currency != "ARS" || resolution.SuggestedCompensation.Unit != "minor" {
		return fmt.Errorf("wrong suggested compensation")
	}
	return s.claimProjectionPrivate()
}
func (s *testSuite) claimProjectionPrivate() error {
	response, err := s.claimResponse()
	if err != nil {
		return err
	}
	if s.adminClaims.selected != "" {
		return s.adminClaimProjectionPrivate()
	}
	for _, field := range []string{"claimant_id", "auth_id", "operator", "operator_id", "actions", "audit", "transactions", "bucket", "key", "credentials", "executed"} {
		if _, exists := response[field]; exists {
			return fmt.Errorf("private field %s exposed", field)
		}
	}
	for _, fixture := range s.claims.claims {
		if s.claimAuth(fixture.owner) != s.currentAuth0ID {
			for _, imageID := range fixture.claim.ImageFileIDs {
				if strings.Contains(string(s.lastBody), imageID) {
					return fmt.Errorf("foreign evidence exposed")
				}
			}
		}
	}
	return nil
}
func (s *testSuite) claimLinkedImage(email, label, imageLabel string) error {
	if err := s.claimUploadImage(email, imageLabel, "image/png"); err != nil {
		return err
	}
	return s.fixtureClaim(email, label, "S1", uuid.NewString(), "damage", "Private evidence testimony", []string{s.claims.images[imageLabel]}, claim.StatusOpen, s.clock.Now())
}
func (s *testSuite) claimTwoOwners(firstEmail, firstLabel, secondEmail, secondLabel string) error {
	if err := s.claimFixtureKey(firstEmail, firstLabel, uuid.NewString()); err != nil {
		return err
	}
	return s.claimFixtureKey(secondEmail, secondLabel, uuid.NewString())
}
func (s *testSuite) claimImageLinkedOnlyTo(imageLabel, linkedLabel, otherLabel string) error {
	fixture := s.claims.claims[linkedLabel]
	if err := s.claimUploadImage(fixture.owner, imageLabel, "image/png"); err != nil {
		return err
	}
	input := fixture.input
	input.ImageFileIDs = []string{s.claims.images[imageLabel]}
	normalized, err := input.Normalize()
	if err != nil {
		return err
	}
	if err := (testsupport.ClaimLifecycleFixture{DB: s.database}).SetEvidence(context.Background(), fixture.claim, normalized.ImageFileIDs); err != nil {
		return err
	}
	fixture.input = normalized
	if err := s.claimSnapshot(linkedLabel); err != nil {
		return err
	}
	if !fixture.claim.MatchesSubmission(normalized) || slices.Contains(s.claims.claims[otherLabel].claim.ImageFileIDs, s.claims.images[imageLabel]) {
		return fmt.Errorf("unexpected evidence links or submission fingerprint")
	}
	return nil
}
func (s *testSuite) claimResponseHasNoURL() error {
	if strings.Contains(string(s.lastBody), `"url":`) {
		return fmt.Errorf("unexpected temporary URL in response")
	}
	return nil
}
func (s *testSuite) claimDetailExcludesImage(imageLabel string) error {
	if s.lastStatus != 200 {
		return fmt.Errorf("detail returned %d", s.lastStatus)
	}
	if strings.Contains(string(s.lastBody), s.claims.images[imageLabel]) {
		return fmt.Errorf("unlinked evidence exposed")
	}
	return s.claimResponseHasNoURL()
}
func (s *testSuite) claimImageAuthorized(imageLabel string) error {
	var response struct {
		Images []struct {
			FileID string `json:"file_id"`
			URL    string `json:"url"`
		}
	}
	if err := json.Unmarshal(s.lastBody, &response); err != nil {
		return err
	}
	if s.lastStatus != 200 || len(response.Images) != 1 || response.Images[0].FileID != s.claims.images[imageLabel] {
		return fmt.Errorf("expected linked private evidence in detail: %s", s.lastBody)
	}
	target, err := url.Parse(response.Images[0].URL)
	if err != nil {
		return err
	}
	if target.Host == "" || target.Query().Get("X-Amz-Expires") == "" || target.Query().Get("X-Amz-Signature") == "" {
		return fmt.Errorf("missing private temporary URL")
	}
	req, err := http.NewRequest(http.MethodGet, response.Images[0].URL, nil)
	if err != nil {
		return err
	}
	result, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer result.Body.Close()
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return err
	}
	expected, err := testsupport.ClaimImagePNG()
	if err != nil {
		return err
	}
	if result.StatusCode != 200 || !bytes.Equal(data, expected) {
		return fmt.Errorf("private image not accessible through authorized URL")
	}
	return nil
}
