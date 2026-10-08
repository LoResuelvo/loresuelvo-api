package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

const adminClaimsPath = "/admin/claims"
const claimReasoning = "Se verificó el incumplimiento acordado"

type adminClaimState struct {
	selected, key, correlation string
	payload                    map[string]any
	expectedPayload            map[string]any
	originalAction             json.RawMessage
	originalResolution         json.RawMessage
	mark                       int64
	counts                     map[string]int64
	operation                  *operationDetailAuditSnapshot
	event                      *audit.Event
}

func registerAdminClaimSteps(sc *godog.ScenarioContext, s *testSuite) {
	sc.Step(`^que "([^"]*)" tiene el reclamo "([^"]*)" en estado "([^"]*)"(?: sobre (?:(?:la solicitud de trabajo |la operación )?"([^"]*)"))?$`, s.adminClaimFixture)
	sc.Step(`^"([^"]*)" tiene el reclamo "([^"]*)" sobre la operación de la solicitud "([^"]*)"$`, func(email, label, request string) error { return s.adminClaimFixture(email, label, "open", request) })
	sc.Step(`^"([^"]*)" tiene el reclamo "([^"]*)" sobre la operación de la propuesta posterior "([^"]*)" de "([^"]*)"$`, s.adminClaimProposalFixture)
	sc.Step(`^que "([^"]*)" tiene el reclamo(?: abierto "([^"]*)"| "([^"]*)" abierto sobre la operación "([^"]*)")$`, func(email, first, second, request string) error {
		if first != "" {
			second = first
		}
		return s.adminClaimFixture(email, second, "open", request)
	})
	sc.Step(`^que "([^"]*)" presentó el reclamo abierto "([^"]*)" sobre "([^"]*)" con motivo "([^"]*)" y testimonio "([^"]*)"$`, func(email, label, request, reason, text string) error {
		return s.fixtureClaim(email, label, request, uuid.NewString(), reason, text, nil, claim.StatusOpen, s.clock.Now())
	})
	sc.Step(`^que "([^"]*)" tiene el reclamo "([^"]*)" sobre "([^"]*)" en estado "([^"]*)", reportado el "([^"]*)", con motivo "([^"]*)" y testimonio "([^"]*)"$`, func(email, label, request, status, instant, reason, text string) error {
		now, err := time.Parse(time.RFC3339, instant)
		if err != nil {
			return err
		}
		return s.fixtureClaim(email, label, request, uuid.NewString(), reason, text, nil, claim.Status(status), now)
	})
	sc.Step(`^"([^"]*)" tiene los reclamos abiertos "([^"]*)", "([^"]*)" y "([^"]*)" reportados respectivamente el "([^"]*)", "([^"]*)" y "([^"]*)", con "([^"]*)", "([^"]*)" y "([^"]*)" vinculados a las operaciones distintas "([^"]*)", "([^"]*)" y "([^"]*)", y con el ID persistido de "([^"]*)" mayor que el de "([^"]*)"$`, s.adminClaimPageFixture)
	sc.Step(`^"([^"]*)" tiene dos evidencias vinculadas$`, s.adminClaimTwoImages)
	sc.Step(`^que "([^"]*)" tiene un reclamo por cada estado del expediente: open, in_review, resolved y dismissed, vinculados respectivamente a las operaciones distintas "([^"]*)", "([^"]*)", "([^"]*)" y "([^"]*)"$`, s.adminClaimStatesFixture)
	sc.Step(`^que "([^"]*)" tiene el reclamo finalizado "([^"]*)" con su dictamen formal persistido por el contrato compartido con US-31$`, s.adminClaimFinalFixture)
	sc.Step(`^que el dictamen de "([^"]*)" tiene tipo "consumer_favor", fundamentación "([^"]*)" y fecha "([^"]*)"$`, s.claimResolutionFixture)
	sc.Step(`^que el expediente conserva su historial de actuaciones y estados$`, s.adminClaimLegalHistory)
	sc.Step(`^que "([^"]*)" tiene el reclamo "([^"]*)" sin evidencias vinculadas$`, func(email, label string) error { return s.adminClaimFixture(email, label, "open", "S1") })
	sc.Step(`^que "([^"]*)" tiene el reclamo "([^"]*)" y "([^"]*)" tiene el reclamo "([^"]*)" sobre la misma operación$`, s.claimTwoOwners)
	sc.Step(`^la imagen privada confirmada "([^"]*)" está vinculada a "([^"]*)" y no a "([^"]*)"$`, s.claimImageLinkedOnlyTo)
	sc.Step(`^que "([^"]*)" tiene la imagen privada confirmada "([^"]*)" vinculada$`, s.adminClaimLinkImage)
	sc.Step(`^que "([^"]*)" conserva la identidad canónica "([^"]*)" seguida del ID persistido de "([^"]*)" y la referencia "([^"]*)" de "([^"]*)"$`, s.adminClaimCanonicalFixture)
	sc.Step(`^que "([^"]*)" conserva su referencia original a la solicitud de trabajo "([^"]*)"$`, s.adminClaimRequestReference)
	sc.Step(`^el reclamo conserva la referencia original a la solicitud de trabajo "([^"]*)"$`, func(request string) error { return s.adminClaimRequestReference(s.adminClaims.selected, request) })
	sc.Step(`^el reclamo "([^"]*)" conserva la fundamentación original y la fecha del dictamen$`, s.adminClaimResolutionBaseline)
	sc.Step(`^que el administrador "([^"]*)" inició la revisión de "([^"]*)" con la clave "([^"]*)" en el instante "([^"]*)"$`, s.adminClaimHistoricalReview)
	sc.Step(`^el administrador "([^"]*)" finalizó luego "([^"]*)" con el dictamen "([^"]*)" y fundamentación "([^"]*)" en el instante "([^"]*)"$`, s.adminClaimHistoricalResolution)
	sc.Step(`^que el administrador "([^"]*)" registró para "([^"]*)" el dictamen final "([^"]*)" con la clave "([^"]*)" en el instante "([^"]*)"$`, s.adminClaimHistoricalKeyResolution)
	sc.Step(`^que el administrador "([^"]*)" ya usó la clave "([^"]*)" para (.*)$`, s.adminClaimHistoricalUse)
	sc.Step(`^falla el registro de auditoría de acceso al reclamo "([^"]*)"$`, func(label string) error { s.adminClaimCapture.fail = true; return s.claimSnapshot(label) })
	sc.Step(`^consulto la bandeja administrativa de reclamos$`, func() error { return s.adminClaimList(nil) })
	sc.Step(`^consulto la bandeja administrativa de reclamos en la página (\d+) con límite 2 y estado "([^"]*)"$`, func(page int, status string) error {
		return s.adminClaimList(url.Values{"page": {strconv.Itoa(page)}, "limit": {"2"}, "status": {status}})
	})
	sc.Step(`^consulto la bandeja administrativa de reclamos con estado "([^"]*)"$`, func(status string) error { return s.adminClaimList(url.Values{"status": {status}}) })
	sc.Step(`^consulto la bandeja administrativa de reclamos con búsqueda "([^"]*)" y estado "([^"]*)"$`, func(query, status string) error {
		return s.adminClaimList(url.Values{"q": {query}, "status": {status}})
	})
	sc.Step(`^consulto la bandeja administrativa de reclamos con (el estado "[^"]*"|la página \d+|el límite \d+)$`, s.adminClaimInvalidList)
	sc.Step(`^busco reclamos por "([^"]*)" en la bandeja administrativa$`, s.adminClaimSearch)
	sc.Step(`^consulto el detalle administrativo del reclamo "([^"]*)"(?: con la correlación "([^"]*)")?$`, s.adminClaimGet)
	sc.Step(`^inicio la revisión administrativa del reclamo "([^"]*)" con la clave "([^"]*)" y la correlación "([^"]*)"$`, s.adminClaimReview)
	sc.Step(`^intento iniciar la revisión administrativa del reclamo "([^"]*)" con una clave nueva$`, func(label string) error { return s.adminClaimReview(label, uuid.NewString(), uuid.NewString()) })
	sc.Step(`^intento iniciar la revisión administrativa de "([^"]*)" con (.*)$`, func(label, input string) error {
		key := ""
		if strings.Contains(input, "no-es-uuid") {
			key = "no-es-uuid"
		}
		return s.adminClaimReview(label, key, uuid.NewString())
	})
	sc.Step(`^repito el inicio de revisión de "([^"]*)" con la misma clave y el mismo contenido$`, func(label string) error { return s.adminClaimReview(label, s.adminClaims.key, uuid.NewString()) })
	sc.Step(`^registro para "([^"]*)" el dictamen de tipo "([^"]*)" con fundamentación "([^"]*)", la clave nueva "([^"]*)" y la correlación "([^"]*)"$`, s.adminClaimResolve)
	sc.Step(`^registro para "([^"]*)" un dictamen "([^"]*)" con fundamentación "([^"]*)" y una compensación sugerida de (\d+) centavos con la clave nueva "([^"]*)"$`, func(label, kind, text string, amount int, key string) error {
		payload := resolutionPayload(kind, text)
		payload["suggested_compensation"] = map[string]any{"amount_minor": amount}
		return s.adminClaimWrite(label, "resolution", key, uuid.NewString(), payload)
	})
	sc.Step(`^intento registrar un dictamen válido para "([^"]*)" con una clave nueva$`, func(label string) error {
		return s.adminClaimResolve(label, "consumer_favor", claimReasoning, uuid.NewString(), uuid.NewString())
	})
	sc.Step(`^intento dictaminar "([^"]*)" con (.*) y una clave nueva$`, s.adminClaimInvalidReasoning)
	sc.Step(`^intento registrar para "([^"]*)" una compensación sugerida (.*) con una clave nueva$`, s.adminClaimInvalidCompensation)
	sc.Step(`^intento registrar para "([^"]*)" un dictamen con (.*)$`, s.adminClaimInvalidResolution)
	sc.Step(`^repito el mismo dictamen para "([^"]*)" con la misma clave y contenido$`, func(label string) error {
		return s.adminClaimWrite(label, "resolution", s.adminClaims.key, uuid.NewString(), s.adminClaims.payload)
	})
	sc.Step(`^intento registrar para "([^"]*)" (.*) con la misma clave$`, func(label, input string) error {
		kind := "consumer_favor"
		text := claimReasoning
		if strings.Contains(input, "provider_favor") {
			kind = "provider_favor"
			text = "Different reasoning"
		}
		return s.adminClaimResolve(label, kind, text, s.adminClaims.key, uuid.NewString())
	})
	registerAdminClaimAssertions(sc, s)
}

func resolutionPayload(kind, text string) map[string]any {
	return map[string]any{"type": kind, "reasoning": text}
}
func (s *testSuite) adminClaimFixture(email, label, status, request string) error {
	if request == "" {
		request = "S1"
	}
	now := s.clock.Now()
	if status != "open" {
		now = now.Add(-2 * time.Hour)
	}
	if err := s.fixtureClaim(email, label, request, uuid.NewString(), "non_compliance", "Original testimony", nil, claim.StatusOpen, now); err != nil {
		return err
	}
	return s.adminClaimStateFixture(label, status)
}
func (s *testSuite) adminClaimProposalFixture(email, label, proposal, request string) error {
	auth, permissions := s.currentAuth0ID, s.currentPermissions
	defer func() { s.currentAuth0ID = auth; s.currentPermissions = permissions }()
	s.currentAuth0ID = s.claimAuth(email)
	s.currentPermissions = nil
	if err := s.submitClaimReference("service_proposal", proposal, uuid.NewString(), "non_compliance", "Original testimony", nil); err != nil {
		return err
	}
	if err := s.claimCreated(); err != nil {
		return err
	}
	if err := s.claimCreatedDistinct(label, "open"); err != nil {
		return err
	}
	s.claims.claims[label].owner = email
	return nil
}
func (s *testSuite) adminClaimPageFixture(email, a, b, c, ta, tb, tc, la, lb, lc, ra, rb, rc, higher, lower string) error {
	if a != la || b != lb || c != lc {
		return fmt.Errorf("claim label mapping differs")
	}
	for i, request := range []string{ra, rb, rc} {
		if _, ok := s.operationInbox.requests[request]; !ok {
			if err := s.claimDistinctRequest(request, email, time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)); err != nil {
				return err
			}
		}
		instant, err := time.Parse(time.RFC3339, []string{ta, tb, tc}[i])
		if err != nil {
			return err
		}
		if err := s.fixtureClaim(email, []string{a, b, c}[i], request, uuid.NewString(), "damage", "Original testimony", nil, claim.StatusOpen, instant); err != nil {
			return err
		}
	}
	if s.claims.claims[higher].claim.ID <= s.claims.claims[lower].claim.ID {
		return fmt.Errorf("fixture IDs not ordered")
	}
	return nil
}
func (s *testSuite) adminClaimStatesFixture(email, a, b, c, d string) error {
	for i, request := range []string{a, b, c, d} {
		if _, ok := s.operationInbox.requests[request]; !ok {
			if err := s.claimDistinctRequest(request, email, time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)); err != nil {
				return err
			}
		}
		if err := s.adminClaimFixture(email, fmt.Sprintf("C%d", i+1), []string{"open", "in_review", "resolved", "dismissed"}[i], request); err != nil {
			return err
		}
	}
	return nil
}
func (s *testSuite) adminClaimLinkImage(label, image string) error {
	f := s.claims.claims[label]
	if err := s.claimUploadImage(f.owner, image, "image/png"); err != nil {
		return err
	}
	ids := append(append([]string{}, f.claim.ImageFileIDs...), s.claims.images[image])
	if err := (testsupport.ClaimLifecycleFixture{DB: s.database}).SetEvidence(context.Background(), f.claim, ids); err != nil {
		return err
	}
	f.input.ImageFileIDs = ids
	return s.claimSnapshot(label)
}
func (s *testSuite) adminClaimTwoImages(label string) error {
	for _, image := range []string{"IMG1", "IMG2"} {
		if err := s.adminClaimLinkImage(label, image); err != nil {
			return err
		}
	}
	return nil
}
func (s *testSuite) adminClaimCanonicalFixture(label, prefix, operation, kind, reference string) error {
	f := s.claims.claims[label].claim
	id := s.operationInbox.requests[operation].id
	refID := s.operationInbox.requests[reference].id
	if prefix == "sp-" {
		id = s.operationInbox.proposals[operation].id
	}
	if kind == "service_proposal_id" {
		refID = s.operationInbox.proposals[reference].id
	}
	if fmt.Sprintf("%s-%d", f.OperationID.Kind, f.OperationID.ResourceID) != prefix+strconv.Itoa(id) || string(f.Reference.Kind())+"_id" != kind || f.Reference.ID() != strconv.Itoa(refID) {
		return fmt.Errorf("canonical reference mismatch")
	}
	return nil
}
func (s *testSuite) adminClaimRequestReference(label, request string) error {
	f := s.claims.claims[label]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	if found.Reference.Kind() != claim.ReferenceKindJobRequest || found.Reference.ID() != strconv.Itoa(s.operationInbox.requests[request].id) {
		return fmt.Errorf("original request reference changed")
	}
	return nil
}
func (s *testSuite) adminClaimLegalHistory() error {
	f := s.claims.claims["C1"].claim
	if len(f.Actions) != 3 || f.Actions[0].Type != "submitted" || f.Actions[1].Type != "review_started" || f.Actions[2].Type != "resolved" {
		return fmt.Errorf("incomplete persisted lifecycle history")
	}
	return nil
}
func (s *testSuite) adminClaimBaseline(label string) error {
	s.adminClaims.selected = label
	s.claims.selected = label
	mark, err := s.dependencies.Persistence.AuditEventRepository.CaptureWatermark(context.Background())
	if err != nil {
		return err
	}
	s.adminClaims.mark = mark
	counts, err := testsupport.OperationDetailBusinessCounts(context.Background(), s.database)
	if err != nil {
		return err
	}
	s.adminClaims.counts = counts
	s.adminClaimCapture.ids = nil
	s.adminClaimCapture.verified = nil
	s.adminClaimCapture.pending = nil
	for request := range s.operationInbox.requests {
		snapshot, err := s.captureOperationDetailAuditSnapshot(request)
		if err != nil {
			return err
		}
		s.adminClaims.operation = snapshot
		break
	}
	return nil
}
func (s *testSuite) adminClaimList(query url.Values) error {
	mark, err := s.dependencies.Persistence.AuditEventRepository.CaptureWatermark(context.Background())
	if err != nil {
		return err
	}
	s.adminClaims.mark = mark
	return s.sendAdminGet(adminClaimsPath, query, uuid.NewString())
}
func (s *testSuite) adminClaimSearch(query string) error {
	if strings.HasPrefix(query, "jr- seguido") {
		query = "jr-" + strconv.Itoa(s.operationInbox.requests["S1"].id)
	}
	if strings.HasPrefix(query, "sp- seguido") {
		query = "sp-" + strconv.Itoa(s.operationInbox.proposals["P2"].id)
	}
	return s.adminClaimList(url.Values{"q": {query}})
}
func (s *testSuite) adminClaimInvalidList(input string) error {
	q := url.Values{}
	switch input {
	case `el estado "pending"`:
		q.Set("status", "pending")
	case "la página 0":
		q.Set("page", "0")
	case "el límite 0":
		q.Set("limit", "0")
	case "el límite 101":
		q.Set("limit", "101")
	default:
		return fmt.Errorf("unknown list input %q", input)
	}
	return s.adminClaimList(q)
}
func (s *testSuite) adminClaimGet(label, correlation string) error {
	if err := s.adminClaimBaseline(label); err != nil {
		return err
	}
	if correlation == "" {
		correlation = uuid.NewString()
	}
	s.adminClaims.correlation = correlation
	return s.sendAdminGet(adminClaimsPath+"/"+strconv.Itoa(s.claims.claims[label].claim.ID), nil, correlation)
}
func (s *testSuite) adminClaimReview(label, key, correlation string) error {
	return s.adminClaimWrite(label, "review", key, correlation, nil)
}
func (s *testSuite) adminClaimResolve(label, kind, text, key, correlation string) error {
	return s.adminClaimWrite(label, "resolution", key, correlation, resolutionPayload(kind, text))
}
func (s *testSuite) adminClaimWrite(label, action, key, correlation string, payload map[string]any) error {
	if action == "review" && payload == nil {
		payload = map[string]any{}
	}
	if err := s.adminClaimBaseline(label); err != nil {
		return err
	}
	s.adminClaims.correlation = correlation
	s.adminClaims.expectedPayload = payload
	s.adminRequest.sentHeaders = http.Header{}
	s.adminRequest.sentHeaders.Set("X-Request-ID", correlation)
	return s.claimSend(http.MethodPost, adminClaimsPath+"/"+strconv.Itoa(s.claims.claims[label].claim.ID)+"/"+action, key, payload)
}
func (s *testSuite) adminClaimHistoricalReview(email, label, key, instant string) error {
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	if err := s.setInboxFixtureClock(now); err != nil {
		return err
	}
	if err := s.iAmAuthenticatedAsAdminWithPermission(email, "write:admin_claims"); err != nil {
		return err
	}
	if err := s.adminClaimReview(label, key, uuid.NewString()); err != nil {
		return err
	}
	if s.lastStatus != 200 {
		return fmt.Errorf("historical review returned %d: %s", s.lastStatus, s.lastBody)
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	s.adminClaims.originalAction = append(json.RawMessage{}, raw["action"]...)
	s.adminClaims.key = key
	return s.claimSnapshot(label)
}
func (s *testSuite) adminClaimHistoricalResolution(email, label, kind, text, instant string) error {
	return s.adminClaimHistoricalKeyResolutionWithText(email, label, kind, text, uuid.NewString(), instant)
}
func (s *testSuite) adminClaimHistoricalKeyResolution(email, label, kind, key, instant string) error {
	return s.adminClaimHistoricalKeyResolutionWithText(email, label, kind, claimReasoning, key, instant)
}
func (s *testSuite) adminClaimHistoricalKeyResolutionWithText(email, label, kind, text, key, instant string) error {
	reviewKey, reviewAction := s.adminClaims.key, s.adminClaims.originalAction
	now, err := time.Parse(time.RFC3339, instant)
	if err != nil {
		return err
	}
	if err := s.setInboxFixtureClock(now); err != nil {
		return err
	}
	if err := s.iAmAuthenticatedAsAdminWithPermission(email, "write:admin_claims"); err != nil {
		return err
	}
	if err := s.adminClaimResolve(label, kind, text, key, uuid.NewString()); err != nil {
		return err
	}
	if s.lastStatus != 200 {
		return fmt.Errorf("historical resolution returned %d: %s", s.lastStatus, s.lastBody)
	}
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	s.adminClaims.originalResolution = append(json.RawMessage{}, raw["resolution"]...)
	if reviewKey != "" {
		s.adminClaims.key = reviewKey
		s.adminClaims.originalAction = reviewAction
	} else {
		s.adminClaims.key = key
		s.adminClaims.originalAction = append(json.RawMessage{}, raw["action"]...)
	}
	s.adminClaims.payload = resolutionPayload(kind, text)
	return s.claimSnapshot(label)
}
func (s *testSuite) adminClaimHistoricalUse(email, key, input string) error {
	label := "C1"
	if strings.Contains(input, `"C2"`) {
		label = "C2"
	}
	if strings.Contains(input, "revisión") {
		if err := s.adminClaimPrepareState(label, claim.StatusOpen, nil); err != nil {
			return err
		}
		if err := s.claimSnapshot(label); err != nil {
			return err
		}
		if err := s.adminClaimHistoricalReview(email, label, key, s.clock.Now().Format(time.RFC3339)); err != nil {
			return err
		}
	} else {
		if err := s.adminClaimPrepareState(label, claim.StatusInReview, nil); err != nil {
			return err
		}
		if err := s.claimSnapshot(label); err != nil {
			return err
		}
		s.adminClaims.key = ""
		if err := s.adminClaimHistoricalKeyResolution(email, label, "consumer_favor", key, s.clock.Now().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	for label := range s.claims.claims {
		if err := s.claimSnapshot(label); err != nil {
			return err
		}
	}
	return nil
}
func (s *testSuite) adminClaimResolutionBaseline(label string) error {
	raw, err := s.claimResponse()
	if err != nil {
		return err
	}
	s.adminClaims.originalResolution = append(json.RawMessage{}, raw["resolution"]...)
	return s.claimSnapshot(label)
}
func (s *testSuite) adminClaimInvalidReasoning(label, input string) error {
	p := resolutionPayload("consumer_favor", "")
	switch input {
	case "la fundamentación vacía":
	case `una fundamentación de 5001 caracteres Unicode "ñ"`:
		p["reasoning"] = strings.Repeat("ñ", 5001)
	case "sin el campo de fundamentación":
		delete(p, "reasoning")
	default:
		return fmt.Errorf("unknown reasoning input %q", input)
	}
	return s.adminClaimWrite(label, "resolution", uuid.NewString(), uuid.NewString(), p)
}
func (s *testSuite) adminClaimInvalidCompensation(label, input string) error {
	var amount any
	switch input {
	case "de 0 centavos":
		amount = 0
	case "de -1 centavo":
		amount = -1
	case "de 1.5 centavos":
		amount = json.Number("1.5")
	case "superior al máximo entero de 64 bits":
		amount = json.Number("9223372036854775808")
	case "sin importe":
	default:
		return fmt.Errorf("unknown compensation input %q", input)
	}
	p := resolutionPayload("agreement", claimReasoning)
	c := map[string]any{}
	if amount != nil {
		c["amount_minor"] = amount
	}
	p["suggested_compensation"] = c
	return s.adminClaimWrite(label, "resolution", uuid.NewString(), uuid.NewString(), p)
}
func (s *testSuite) adminClaimInvalidResolution(label, input string) error {
	p := resolutionPayload("consumer_favor", claimReasoning)
	key := uuid.NewString()
	switch input {
	case `el tipo no admitido "neutral", fundamentación válida y clave nueva`:
		p["type"] = "neutral"
	case "el tipo ausente, fundamentación válida y clave nueva":
		delete(p, "type")
	case `tipo "consumer_favor", fundamentación válida y clave ausente`:
		key = ""
	case `tipo "consumer_favor", fundamentación válida y clave "no-es-uuid"`:
		key = "no-es-uuid"
	default:
		return fmt.Errorf("unknown resolution input %q", input)
	}
	return s.adminClaimWrite(label, "resolution", key, uuid.NewString(), p)
}

func (s *testSuite) adminClaimProjectionPrivate() error {
	var response any
	if err := json.Unmarshal(s.lastBody, &response); err != nil {
		return err
	}
	var visit func(any) error
	visit = func(value any) error {
		switch v := value.(type) {
		case map[string]any:
			for key, value := range v {
				switch key {
				case "auth_id", "credentials", "bucket", "key", "transactions", "messages", "payment_transactions":
					return fmt.Errorf("private field %s exposed", key)
				}
				if err := visit(value); err != nil {
					return err
				}
			}
		case []any:
			for _, value := range v {
				if err := visit(value); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(response)
}
func (s *testSuite) adminClaimOriginalFields() error {
	f := s.claims.claims[s.adminClaims.selected]
	found, err := s.claimRepo().FindOwnedByID(context.Background(), f.claim.ClaimantID, f.claim.ID)
	if err != nil {
		return err
	}
	if found.ID != f.snapshot.ID || found.ClaimantID != f.snapshot.ClaimantID || found.ClaimantParty != f.snapshot.ClaimantParty || found.OperationID != f.snapshot.OperationID || found.Reference != f.snapshot.Reference || found.Reason != f.snapshot.Reason || found.Description != f.snapshot.Description || !found.CreatedOn.Equal(f.snapshot.CreatedOn) || !reflect.DeepEqual(found.ImageFileIDs, f.snapshot.ImageFileIDs) {
		return fmt.Errorf("original claim fields changed")
	}
	return nil
}

func (s *testSuite) adminClaimPrepareState(label string, status claim.Status, resolution *claim.Resolution) error {
	operator, err := s.userRepository.FindOperatorIDByAuthID(context.Background(), auth0IDForAdminEmail("operador@example.com"))
	if err != nil {
		return err
	}
	return (testsupport.ClaimLifecycleFixture{DB: s.database, OperatorID: operator}).SetState(context.Background(), s.claims.claims[label].claim.ID, status, resolution)
}
func (s *testSuite) adminClaimStateFixture(label, status string) error {
	f := s.claims.claims[label]
	var resolution *claim.Resolution
	if status == "resolved" || status == "dismissed" {
		kind := claim.ResolutionTypeConsumerFavor
		if status == "dismissed" {
			kind = claim.ResolutionTypeWithoutMerit
		}
		resolution = &claim.Resolution{Type: kind, Reasoning: claimReasoning, ResolvedOn: f.claim.CreatedOn.Add(time.Hour)}
	}
	if err := s.adminClaimPrepareState(label, claim.Status(status), resolution); err != nil {
		return err
	}
	return s.claimSnapshot(label)
}
func (s *testSuite) adminClaimFinalFixture(email, label string) error {
	if err := s.claimFinalFixture(email, label); err != nil {
		return err
	}
	f := s.claims.claims[label]
	if err := s.adminClaimPrepareState(label, f.claim.Status, f.claim.Resolution); err != nil {
		return err
	}
	return s.claimSnapshot(label)
}
