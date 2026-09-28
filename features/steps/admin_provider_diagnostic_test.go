package steps_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	paymentaccount "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment_account"
	"github.com/cucumber/godog"
)

const providerDiagnosticPath = "/admin/providers"

var errInjectedProviderDiagnosticRead = errors.New("injected provider diagnostic read failure")
var errInjectedProviderDiagnosticAudit = errors.New("injected provider diagnostic audit failure")

type providerDiagnosticTestCapture struct {
	failRead      bool
	readAttempts  int
	auditAttempts int
	failAudit     bool
}

func (capture *providerDiagnosticTestCapture) reset() {
	capture.failRead = false
	capture.readAttempts = 0
	capture.auditAttempts = 0
	capture.failAudit = false
}

type providerDiagnosticReaderDecorator struct {
	inner   admin.ProviderDiagnosticReader
	capture *providerDiagnosticTestCapture
}

func (reader providerDiagnosticReaderDecorator) FindByProviderID(ctx context.Context, id int) (*readmodel.ProviderDiagnostic, error) {
	reader.capture.readAttempts++
	if reader.capture.failRead {
		return nil, errInjectedProviderDiagnosticRead
	}
	return reader.inner.FindByProviderID(ctx, id)
}

type providerDiagnosticAuditWriterDecorator struct {
	inner   audit.Writer
	capture *providerDiagnosticTestCapture
}

func (writer providerDiagnosticAuditWriterDecorator) Save(ctx context.Context, event *audit.Event) error {
	writer.capture.auditAttempts++
	if writer.capture.failAudit {
		return errInjectedProviderDiagnosticAudit
	}
	return writer.inner.Save(ctx, event)
}

type providerDiagnosticResponse struct {
	Provider adminProviderDirectoryResponse    `json:"provider"`
	Checks   []providerDiagnosticCheckResponse `json:"checks"`
	Payment  struct {
		State          string     `json:"state"`
		TokenExpiresOn *time.Time `json:"token_expires_on"`
	} `json:"payment"`
	Calendar struct {
		State string `json:"state"`
	} `json:"calendar"`
}

type providerDiagnosticCheckResponse struct {
	Control    string     `json:"control"`
	Result     string     `json:"result"`
	ReasonCode string     `json:"reason_code"`
	EvidenceOn *time.Time `json:"evidence_on"`
}

type providerDiagnosticState struct {
	correlation string
	providerID  int
	operatorID  int
	readMark    int
	auditMark   int
}

func registerAdminProviderDiagnosticSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^consulto el diagnóstico administrativo del prestador "([^\"]*)"(?: con correlación "([^\"]*)")?$`, suite.queryAdminProviderDiagnostic)
	sc.Step(`^intento consultar el diagnóstico administrativo del prestador "([^\"]*)"$`, suite.attemptAdminProviderDiagnostic)
	sc.Step(`^que estoy autenticado como administrador "([^\"]*)" solamente con "([^\"]*)"$`, suite.iAmAuthenticatedAsAdminWithPermission)
	sc.Step(`^el diagnóstico identifica al prestador por su ID, rol, nombre, apellido, correo y fecha de registro, e incluye la URL pública de la foto de perfil persistida$`, suite.diagnosticIncludesProviderDirectoryData)
	sc.Step(`^el diagnóstico incluye el rubro "([^\"]*)" y las zonas "([^\"]*)" y "([^\"]*)" con sus identificadores y disponibilidad habilitada$`, suite.diagnosticIncludesCategoryAndCoverage)
	sc.Step(`^los controles informan exactamente:$`, suite.diagnosticChecksMatchTable)
	sc.Step(`^el control de conexión informa "([^\"]*)" con reason_code "([^\"]*)"$`, suite.diagnosticPaymentConnectionCheck)
	sc.Step(`^el control "([^\"]*)" informa "([^\"]*)" con reason_code "([^\"]*)" y evidencia "([^\"]*)"$`, suite.diagnosticCheckWithEvidence)
	sc.Step(`^la expiración informada es exclusivamente el dato persistido en la cuenta$`, suite.diagnosticPaymentExpiryMatchesPersistedValue)
	sc.Step(`^la evidencia de conexión de pago coincide con el connected_on persistido de "([^\"]*)", sin fijar su fecha en el fixture$`, suite.diagnosticPaymentEvidenceMatchesPersistedValue)
	sc.Step(`^las fechas de evidencia sólo aparecen cuando existe un instante persistido para ese control$`, suite.diagnosticEvidenceIsOptionalOnlyWhenAbsent)
	sc.Step(`^la expiración informada corresponde a la fecha guardada en la cuenta$`, suite.diagnosticPaymentExpiryMatchesPersistedValue)
	sc.Step(`^la respuesta no contiene un veredicto booleano general de aptitud, credenciales, tokens, secretos, sesiones privadas, documentos ni códigos de riesgo$`, suite.diagnosticDoesNotExposeSensitiveData)
	sc.Step(`^que existe una cuenta de Mercado Pago "([^\"]*)" vinculada al prestador "([^\"]*)" con expiración persistida "([^\"]*)"$`, suite.providerDiagnosticPaymentAccountWithExpiry)
	sc.Step(`^antes de entregar el diagnóstico queda preparado exactamente un evento de acceso al recurso "([^\"]*)" con el ID persistido de "([^\"]*)", el operador "([^\"]*)" y la correlación "([^\"]*)"$`, suite.providerDiagnosticAuditPreparedOnce)
	sc.Step(`^el resultado del evento de acceso es "prepared", no "succeeded", y no contiene datos del diagnóstico ni una justificación manual$`, suite.providerDiagnosticAuditContainsNoDiagnosticData)
	sc.Step(`^que falla la lectura de una fuente requerida del diagnóstico del prestador "([^\"]*)"$`, suite.failProviderDiagnosticRead)
	sc.Step(`^que falla el almacenamiento del evento de auditoría de este diagnóstico$`, suite.failProviderDiagnosticAudit)
	sc.Step(`^no se entrega un diagnóstico parcial ni se presenta el fallo como conexión ausente$`, suite.providerDiagnosticFailureIsClosed)
	sc.Step(`^no se registra ningún evento de auditoría para esta solicitud$`, suite.providerDiagnosticFailureDoesNotAudit)
	sc.Step(`^no se entrega ningún campo ni colección del diagnóstico$`, suite.providerDiagnosticFailureIsClosed)
	sc.Step(`^no se entrega ningún dato del diagnóstico$`, suite.providerDiagnosticErrorHasNoData)
	sc.Step(`^no se entrega un diagnóstico$`, suite.providerDiagnosticNotFoundHasNoData)
	sc.Step(`^que no existe ningún prestador con ID persistido (\d+)$`, suite.providerDiagnosticMissingProvider)
	sc.Step(`^no se persiste ningún evento de auditoría para esta consulta diagnóstica$`, suite.providerDiagnosticFailureDoesNotAudit)
	sc.Step(`^que "([^\"]*)" tiene Google Calendar conectado desde "([^\"]*)"$`, suite.providerDiagnosticCalendarConnectedFrom)
	sc.Step(`^que la conexión de Google Calendar de "([^\"]*)" está en estado "([^\"]*)" desde "([^\"]*)"$`, suite.providerDiagnosticCalendarHasStateFrom)
	sc.Step(`^que la zona de cobertura "([^\"]*)" se deshabilita después del registro de "([^\"]*)"$`, suite.providerDiagnosticCoverageZoneDisabled)
	sc.Step(`^el diagnóstico incluye "([^\"]*)" con su disponibilidad deshabilitada y conserva "([^\"]*)" como habilitada$`, suite.providerDiagnosticCoverageZoneAvailability)
	sc.Step(`^Calendar informa el estado "([^\"]*)", resultado "([^\"]*)", reason_code "([^\"]*)" y evidencia "([^\"]*)"$`, suite.providerDiagnosticCalendarCheck)
	sc.Step(`^la conexión no se informa como "([^\"]*)" ni como sincronización confirmada de órdenes$`, suite.providerDiagnosticCalendarIsNotDisconnectedOrSync)
}

func (suite *testSuite) queryAdminProviderDiagnostic(identifier, correlation string) error {
	providerID, err := strconv.Atoi(identifier)
	if err != nil {
		providerID, err = suite.providerIDByEmail(identifier)
		if err != nil {
			return err
		}
	}
	if correlation == "" {
		correlation = "provider-diagnostic-test-" + strconv.Itoa(providerID) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	state := providerDiagnosticState{correlation: correlation, providerID: providerID}
	state.readMark = suite.providerDiagnosticCapture.readAttempts
	state.auditMark = suite.providerDiagnosticCapture.auditAttempts
	state.operatorID, err = suite.providerDiagnosticOperatorID()
	if err != nil {
		return err
	}
	suite.providerDiagnostic = state
	return suite.sendAdminGet(providerDiagnosticPath+"/"+url.PathEscape(strconv.Itoa(providerID))+"/diagnostic", nil, correlation)
}

func (suite *testSuite) attemptAdminProviderDiagnostic(identifier string) error {
	providerID, err := strconv.Atoi(identifier)
	if err != nil {
		providerID, err = suite.providerIDByEmail(identifier)
		if err != nil {
			return err
		}
	}
	correlation := "provider-diagnostic-denied-" + strconv.Itoa(providerID) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	operatorID, err := suite.providerDiagnosticOperatorID()
	if err != nil {
		return err
	}
	suite.providerDiagnostic = providerDiagnosticState{correlation: correlation, providerID: providerID, operatorID: operatorID}
	return suite.sendAdminGet(providerDiagnosticPath+"/"+url.PathEscape(strconv.Itoa(providerID))+"/diagnostic", nil, correlation)
}

func (suite *testSuite) providerDiagnosticOperatorID() (int, error) {
	if suite.currentAuth0ID == "" {
		return 0, nil
	}
	operatorID, err := suite.userRepository.FindOperatorIDByAuthID(suite.scenarioContext, suite.currentAuth0ID)
	if err != nil {
		if len(suite.currentPermissions) == 0 {
			return 0, nil
		}
		return 0, fmt.Errorf("resolving provider diagnostic operator for authenticated admin: %w", err)
	}
	return operatorID, nil
}

func (suite *testSuite) diagnosticResponse() (providerDiagnosticResponse, error) {
	if suite.lastStatus != http.StatusOK {
		return providerDiagnosticResponse{}, fmt.Errorf("expected provider diagnostic status 200, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	var response providerDiagnosticResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return providerDiagnosticResponse{}, fmt.Errorf("decoding provider diagnostic response: %w", err)
	}
	if response.Checks == nil {
		return providerDiagnosticResponse{}, fmt.Errorf("provider diagnostic checks are null: %s", suite.lastBody)
	}
	return response, nil
}

func (suite *testSuite) diagnosticIncludesProviderDirectoryData() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	provider := response.Provider
	if provider.ID != suite.providerDiagnostic.providerID || provider.Role != "provider" || strings.TrimSpace(provider.Name) == "" ||
		strings.TrimSpace(provider.Surname) == "" || strings.TrimSpace(provider.Email) == "" || provider.CreatedOn.IsZero() {
		return fmt.Errorf("provider diagnostic has incomplete persisted provider data: %s", suite.lastBody)
	}
	if provider.ProfilePhotoURL == nil || !isPublicHTTPURL(*provider.ProfilePhotoURL) {
		return fmt.Errorf("provider diagnostic has no valid public profile photo URL: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) diagnosticIncludesCategoryAndCoverage(categoryName, firstZone, secondZone string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Provider.Category.Name != categoryName {
		return fmt.Errorf("expected diagnostic category %q, got %q", categoryName, response.Provider.Category.Name)
	}
	if response.Provider.Category.ID <= 0 {
		return fmt.Errorf("diagnostic category has no persisted identifier")
	}
	zones := make(map[string]adminCoverageZoneResponse, len(response.Provider.CoverageZones))
	for _, zone := range response.Provider.CoverageZones {
		zones[zone.Name] = zone
	}
	for _, name := range []string{firstZone, secondZone} {
		zone, ok := zones[name]
		if !ok || zone.ID <= 0 || zone.MarketID <= 0 || !zone.Enabled {
			return fmt.Errorf("expected enabled diagnostic coverage zone %q with persisted identifiers, got %s", name, suite.lastBody)
		}
	}
	return nil
}

func (suite *testSuite) diagnosticChecksMatchTable(table *godog.Table) error {
	if err := requireTableHeaders(table, "control", "resultado", "reason_code", "evidence_on"); err != nil {
		return err
	}
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if len(response.Checks) != len(table.Rows)-1 {
		return fmt.Errorf("expected exactly %d diagnostic checks, got %d", len(table.Rows)-1, len(response.Checks))
	}
	for i, row := range table.Rows[1:] {
		if len(row.Cells) != 4 {
			return fmt.Errorf("expected four diagnostic assertion cells, got %d", len(row.Cells))
		}
		got := response.Checks[i]
		if got.Control != row.Cells[0].Value || got.Result != row.Cells[1].Value || got.ReasonCode != row.Cells[2].Value {
			return fmt.Errorf("diagnostic check %d mismatch: got %+v, want %q/%q/%q", i+1, got, row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value)
		}
		if expected := row.Cells[3].Value; expected != "valor persistido de connected_on" {
			if err := assertOptionalInstant(got.EvidenceOn, expected); err != nil {
				return fmt.Errorf("diagnostic check %q evidence: %w", got.Control, err)
			}
		} else if got.EvidenceOn == nil {
			return fmt.Errorf("diagnostic check %q has no persisted connected_on evidence", got.Control)
		}
	}
	return nil
}

func (suite *testSuite) diagnosticPaymentConnectionCheck(expectedResult, expectedReason string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	check := findDiagnosticCheck(response.Checks, "payment_account_connection")
	if check == nil || check.Result != expectedResult || check.ReasonCode != expectedReason {
		return fmt.Errorf("unexpected payment connection check: %+v", check)
	}
	return nil
}

func (suite *testSuite) diagnosticCheckWithEvidence(control, expectedResult, expectedReason, evidence string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	check := findDiagnosticCheck(response.Checks, control)
	if check == nil || check.Result != expectedResult || check.ReasonCode != expectedReason {
		return fmt.Errorf("unexpected diagnostic check %q: %+v", control, check)
	}
	return assertOptionalInstant(check.EvidenceOn, evidence)
}

func (suite *testSuite) providerDiagnosticCoverageZoneDisabled(zoneName, providerEmail string) error {
	if _, err := suite.providerIDByEmail(providerEmail); err != nil {
		return err
	}
	return suite.coverageZoneIsDisabled(zoneName)
}

func (suite *testSuite) providerDiagnosticCoverageZoneAvailability(disabledName, enabledName string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	zones := make(map[string]adminCoverageZoneResponse, len(response.Provider.CoverageZones))
	for _, zone := range response.Provider.CoverageZones {
		zones[zone.Name] = zone
	}
	if zone, ok := zones[disabledName]; !ok || zone.Enabled {
		return fmt.Errorf("expected coverage zone %q to remain selected but report disabled", disabledName)
	}
	if zone, ok := zones[enabledName]; !ok || !zone.Enabled {
		return fmt.Errorf("expected coverage zone %q to report enabled", enabledName)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticCalendarCheck(state, result, reason, evidence string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Calendar.State != state {
		return fmt.Errorf("expected Calendar state %q, got %q", state, response.Calendar.State)
	}
	check := findDiagnosticCheck(response.Checks, "calendar_connection")
	if check == nil || check.Result != result || check.ReasonCode != reason {
		return fmt.Errorf("unexpected Calendar diagnostic check: %+v", check)
	}
	return assertOptionalInstant(check.EvidenceOn, evidence)
}

func (suite *testSuite) providerDiagnosticCalendarIsNotDisconnectedOrSync(disconnectedState string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	if response.Calendar.State == disconnectedState || response.Calendar.State != calendarconnection.StatusActionRequired {
		return fmt.Errorf("Calendar state %q was incorrectly reported as disconnected or not action-required", response.Calendar.State)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return err
	}
	if _, exists := raw["synchronization"]; exists {
		return fmt.Errorf("diagnostic claims order synchronization that this endpoint does not observe")
	}
	return nil
}

func (suite *testSuite) diagnosticPaymentEvidenceMatchesPersistedValue(accountID string) error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	providerID, err := suite.providerIDByEmail(response.Provider.Email)
	if err != nil {
		return err
	}
	account, err := suite.paymentAccountRepository.FindByProviderID(suite.scenarioContext, providerID, "mercado_pago")
	if err != nil {
		return fmt.Errorf("loading persisted Mercado Pago account %q: %w", accountID, err)
	}
	if account == nil || account.ExternalAccountID() != accountID {
		return fmt.Errorf("expected persisted Mercado Pago account %q", accountID)
	}
	persisted, err := repositories.NewProviderDiagnosticReader(suite.database).FindByProviderID(suite.scenarioContext, providerID)
	if err != nil {
		return fmt.Errorf("loading persisted diagnostic evidence: %w", err)
	}
	if persisted == nil || persisted.Payment.ConnectedOn == nil {
		return fmt.Errorf("persisted payment account has no connected_on evidence")
	}
	check := findDiagnosticCheck(response.Checks, "payment_account_connection")
	if check == nil || check.EvidenceOn == nil || !check.EvidenceOn.Equal(*persisted.Payment.ConnectedOn) {
		return fmt.Errorf("diagnostic payment evidence does not match persisted connected_on: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) diagnosticEvidenceIsOptionalOnlyWhenAbsent() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	persisted, err := repositories.NewProviderDiagnosticReader(suite.database).FindByProviderID(suite.scenarioContext, response.Provider.ID)
	if err != nil {
		return fmt.Errorf("reading persisted diagnostic evidence: %w", err)
	}
	if persisted == nil {
		return fmt.Errorf("persisted provider diagnostic disappeared")
	}
	identityExpected := persisted.Provider.IdentityVerifiedOn
	if persisted.Provider.IdentityVerificationStatus != "approved" {
		identityExpected = persisted.IdentityResultOn
	}
	pairs := []struct {
		control   string
		persisted *time.Time
	}{
		{"identity_verification", identityExpected},
		{"payment_account_connection", persisted.Payment.ConnectedOn},
		{"payment_token_expiry", persisted.Payment.TokenExpiresOn},
		{"calendar_connection", persisted.Calendar.ConnectedOn},
	}
	if persisted.Calendar.Status == calendarconnection.StatusActionRequired {
		pairs[3].persisted = persisted.Calendar.UpdatedOn
	}
	for _, pair := range pairs {
		check := findDiagnosticCheck(response.Checks, pair.control)
		if check == nil {
			return fmt.Errorf("diagnostic is missing check %q", pair.control)
		}
		if (check.EvidenceOn == nil) != (pair.persisted == nil) {
			return fmt.Errorf("check %q evidence presence does not match persisted source", pair.control)
		}
		if pair.persisted != nil && !check.EvidenceOn.Equal(*pair.persisted) {
			return fmt.Errorf("check %q evidence differs from persisted source", pair.control)
		}
	}
	return nil
}

func (suite *testSuite) diagnosticPaymentExpiryMatchesPersistedValue() error {
	response, err := suite.diagnosticResponse()
	if err != nil {
		return err
	}
	persisted, err := repositories.NewProviderDiagnosticReader(suite.database).FindByProviderID(suite.scenarioContext, response.Provider.ID)
	if err != nil {
		return fmt.Errorf("loading persisted payment expiry: %w", err)
	}
	if persisted == nil || persisted.Payment.TokenExpiresOn == nil || response.Payment.TokenExpiresOn == nil || !response.Payment.TokenExpiresOn.Equal(*persisted.Payment.TokenExpiresOn) {
		return fmt.Errorf("diagnostic token expiry does not match the persisted account value: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticPaymentAccountWithExpiry(accountID, email, expiresOnText string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	expiresOn, err := time.Parse(time.RFC3339Nano, expiresOnText)
	if err != nil {
		return fmt.Errorf("parsing persisted Mercado Pago expiry %q: %w", expiresOnText, err)
	}
	paymentProvider, err := paymentaccount.NewPaymentProvider("mercado_pago")
	if err != nil {
		return err
	}
	attempt := paymentaccount.NewAuthorizationAttempt(
		providerID,
		paymentProvider,
		[]byte("01234567890123456789012345678901"),
		[]byte("test-code-verifier-ciphertext"),
		time.Now().UTC().Add(time.Hour),
	)
	if err := suite.dependencies.Persistence.AuthorizationAttemptRepository.Save(suite.scenarioContext, attempt); err != nil {
		return fmt.Errorf("saving Mercado Pago authorization attempt fixture: %w", err)
	}
	account, err := paymentaccount.NewPaymentAccount(
		providerID,
		paymentProvider,
		accountID,
		[]byte("test-access-token-ciphertext"),
		[]byte("test-refresh-token-ciphertext"),
		expiresOn,
	)
	if err != nil {
		return fmt.Errorf("creating Mercado Pago account fixture: %w", err)
	}
	if err := suite.paymentAccountRepository.SaveFromAuthorization(suite.scenarioContext, attempt.ID, account); err != nil {
		return fmt.Errorf("persisting Mercado Pago account fixture: %w", err)
	}
	persisted, err := suite.paymentAccountRepository.FindByProviderID(suite.scenarioContext, providerID, paymentProvider)
	if err != nil {
		return fmt.Errorf("verifying persisted Mercado Pago account fixture: %w", err)
	}
	if persisted == nil || persisted.ExternalAccountID() != accountID || !persisted.TokenExpiresOn().Equal(expiresOn) {
		return fmt.Errorf("persisted Mercado Pago account fixture differs from requested ID/expiry")
	}
	return nil
}

func (suite *testSuite) diagnosticDoesNotExposeSensitiveData() error {
	var response map[string]any
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	forbidden := map[string]bool{"eligible": true, "is_eligible": true, "credentials": true, "access_token": true, "refresh_token": true, "token": true, "secret": true, "session": true, "document": true, "risk_code": true, "risk_codes": true}
	var visit func(any, string) error
	visit = func(value any, path string) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if forbidden[strings.ToLower(key)] {
					return fmt.Errorf("diagnostic response exposes forbidden field %q at %s", key, path)
				}
				if err := visit(child, path+"."+key); err != nil {
					return err
				}
			}
		case []any:
			for i, child := range typed {
				if err := visit(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(response, "response")
}

func (suite *testSuite) providerDiagnosticAuditPreparedOnce(resourceType, email, operatorEmail, correlation string) error {
	if resourceType != "provider" || correlation != suite.providerDiagnostic.correlation {
		return fmt.Errorf("unexpected provider diagnostic audit resource/correlation")
	}
	resourceID := strconv.Itoa(suite.providerDiagnostic.providerID)
	operatorID, err := suite.userRepository.FindOperatorIDByAuthID(suite.scenarioContext, auth0IDForAdminEmail(operatorEmail))
	if err != nil {
		return err
	}
	if operatorID != suite.providerDiagnostic.operatorID {
		return fmt.Errorf("audit operator differs from authenticated administrator")
	}
	persistedID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil || persistedID != suite.providerDiagnostic.providerID || resourceID != strconv.Itoa(persistedID) {
		return fmt.Errorf("audit event resource ID does not match persisted provider ID: %v", err)
	}
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	filter := audit.LogFilter{ResourceType: &resourceType, ResourceID: &resourceID}
	if operatorID > 0 {
		filter.OperatorID = &operatorID
	}
	events, err := reader.FindPage(suite.scenarioContext, filter, watermark, nil, 100)
	if err != nil {
		return err
	}
	matches := 0
	for _, event := range events {
		if event.CorrelationID() != correlation {
			continue
		}
		if event.Action() != audit.ActionAccess || event.Result() != audit.ResultPrepared {
			return fmt.Errorf("diagnostic access audit has unexpected action/result: %s/%s", event.Action(), event.Result())
		}
		matches++
	}
	if matches != 1 {
		return fmt.Errorf("expected exactly one persisted prepared provider access event, got %d", matches)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticAuditContainsNoDiagnosticData() error {
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	resourceType := "provider"
	resourceID := strconv.Itoa(suite.providerDiagnostic.providerID)
	filter := audit.LogFilter{OperatorID: &suite.providerDiagnostic.operatorID, ResourceType: &resourceType, ResourceID: &resourceID}
	events, err := reader.FindPage(suite.scenarioContext, filter, watermark, nil, 100)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.CorrelationID() != suite.providerDiagnostic.correlation {
			continue
		}
		if event.Result() != audit.ResultPrepared || event.Reason() != nil || event.StateChange() != nil {
			return fmt.Errorf("diagnostic audit event includes a result, reason, or state change beyond prepared access")
		}
	}
	return nil
}

func (suite *testSuite) failProviderDiagnosticRead(_ string) error {
	suite.providerDiagnosticCapture.failRead = true
	suite.providerDiagnostic.readMark = suite.providerDiagnosticCapture.readAttempts
	suite.providerDiagnostic.auditMark = suite.providerDiagnosticCapture.auditAttempts
	return nil
}

func (suite *testSuite) failProviderDiagnosticAudit() error {
	suite.providerDiagnosticCapture.failAudit = true
	suite.providerDiagnostic.auditMark = suite.providerDiagnosticCapture.auditAttempts
	return nil
}

func (suite *testSuite) providerDiagnosticMissingProvider(idText string) error {
	id, err := strconv.Atoi(idText)
	if err != nil || id <= 0 {
		return fmt.Errorf("invalid missing provider fixture ID %q", idText)
	}
	suite.providerDiagnostic.providerID = id
	diagnostic, err := repositories.NewProviderDiagnosticReader(suite.database).FindByProviderID(suite.scenarioContext, id)
	if err != nil {
		return fmt.Errorf("checking absent provider fixture: %w", err)
	}
	if diagnostic != nil {
		return fmt.Errorf("provider fixture ID %d unexpectedly exists", id)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticCalendarConnectedFrom(email, connectedOnText string) error {
	return suite.saveProviderDiagnosticCalendarConnection(email, calendarconnection.StatusConnected, connectedOnText)
}

func (suite *testSuite) providerDiagnosticCalendarHasStateFrom(email, state, updatedOnText string) error {
	return suite.saveProviderDiagnosticCalendarConnection(email, state, updatedOnText)
}

func (suite *testSuite) saveProviderDiagnosticCalendarConnection(email, state, persistedOnText string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	persistedOn, err := time.Parse(time.RFC3339Nano, persistedOnText)
	if err != nil {
		return fmt.Errorf("parsing Google Calendar persisted instant %q: %w", persistedOnText, err)
	}
	var connection *calendarconnection.Connection
	if state == calendarconnection.StatusConnected {
		connection, err = calendarconnection.NewConnection(providerID, "primary", []byte("test-refresh-token-ciphertext"), persistedOn)
	} else {
		connection, err = calendarconnection.RehydrateConnection(providerID, []byte("test-refresh-token-ciphertext"), "primary", state, persistedOn, persistedOn)
	}
	if err != nil {
		return fmt.Errorf("creating persisted Google Calendar connection fixture: %w", err)
	}
	if err := suite.calendarConnectionRepository.Save(suite.scenarioContext, connection); err != nil {
		return fmt.Errorf("saving Google Calendar connection fixture: %w", err)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticFailureIsClosed() error {
	if suite.lastStatus != http.StatusInternalServerError {
		return fmt.Errorf("expected diagnostic read failure to return 500, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding diagnostic failure response: %w", err)
	}
	if len(response) != 1 || response["error"] == nil || bytesContainAny(suite.lastBody, []byte(`"provider"`), []byte(`"checks"`), []byte(`"payment"`), []byte(`"calendar"`), []byte(`"disconnected"`)) {
		return fmt.Errorf("diagnostic reader failure leaked partial or misleading connection data: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticErrorHasNoData() error {
	if suite.lastStatus != http.StatusUnauthorized && suite.lastStatus != http.StatusForbidden {
		return fmt.Errorf("expected an authorization error, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding provider diagnostic authorization failure: %w", err)
	}
	if len(response) == 0 || len(response) > 2 || response["error"] == nil {
		return fmt.Errorf("authorization failure included diagnostic data: %s", suite.lastBody)
	}
	for field := range response {
		if field != "error" && field != "message" {
			return fmt.Errorf("authorization failure exposes diagnostic field %q", field)
		}
	}
	return nil
}

func (suite *testSuite) providerDiagnosticNotFoundHasNoData() error {
	if suite.lastStatus != http.StatusNotFound {
		return fmt.Errorf("expected missing diagnostic provider status 404, got %d: %s", suite.lastStatus, suite.lastBody)
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("decoding missing diagnostic provider response: %w", err)
	}
	if len(response) != 1 || response["error"] == nil {
		return fmt.Errorf("missing provider response exposed diagnostic data: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) providerDiagnosticFailureDoesNotAudit() error {
	if suite.providerDiagnosticCapture.failRead && suite.providerDiagnosticCapture.readAttempts != suite.providerDiagnostic.readMark+1 {
		return fmt.Errorf("expected exactly one failing diagnostic read attempt")
	}
	if suite.providerDiagnosticCapture.failAudit {
		if suite.providerDiagnosticCapture.auditAttempts != suite.providerDiagnostic.auditMark+1 {
			return fmt.Errorf("expected exactly one failed diagnostic audit write")
		}
	} else if suite.providerDiagnosticCapture.auditAttempts != suite.providerDiagnostic.auditMark {
		return fmt.Errorf("audit writer was called after the required diagnostic read failed")
	}
	if err := suite.assertNoDiagnosticAuditEvent(suite.providerDiagnostic.correlation, suite.providerDiagnostic.providerID, suite.providerDiagnostic.operatorID); err != nil {
		return err
	}
	return nil
}

func (suite *testSuite) assertNoDiagnosticAuditEvent(correlation string, providerID, _ int) error {
	if correlation == "" || providerID <= 0 {
		return fmt.Errorf("cannot verify diagnostic audit absence without request correlation and provider ID")
	}
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	resourceType := "provider"
	resourceID := strconv.Itoa(providerID)
	filter := audit.LogFilter{ResourceType: &resourceType, ResourceID: &resourceID}
	events, err := reader.FindPage(suite.scenarioContext, filter, watermark, nil, 100)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.CorrelationID() == correlation {
			return fmt.Errorf("unexpected provider diagnostic audit event for failed request")
		}
	}
	return nil
}

func findDiagnosticCheck(checks []providerDiagnosticCheckResponse, control string) *providerDiagnosticCheckResponse {
	for i := range checks {
		if checks[i].Control == control {
			return &checks[i]
		}
	}
	return nil
}

func assertOptionalInstant(actual *time.Time, expected string) error {
	if expected == "" || expected == "null" {
		if actual != nil {
			return fmt.Errorf("expected null evidence, got %s", actual.Format(time.RFC3339Nano))
		}
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, expected)
	if err != nil {
		return fmt.Errorf("invalid expected evidence instant %q: %w", expected, err)
	}
	if actual == nil || !actual.Equal(parsed) {
		return fmt.Errorf("expected evidence %s, got %v", parsed.Format(time.RFC3339Nano), actual)
	}
	return nil
}

func isPublicHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func bytesContainAny(value []byte, candidates ...[]byte) bool {
	for _, candidate := range candidates {
		if strings.Contains(string(value), string(candidate)) {
			return true
		}
	}
	return false
}
