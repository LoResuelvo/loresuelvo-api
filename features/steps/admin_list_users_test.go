package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type adminConsumerDirectoryResponse struct {
	ID              int       `json:"id"`
	Role            string    `json:"role"`
	Name            string    `json:"name"`
	Surname         string    `json:"surname"`
	Email           string    `json:"email"`
	ProfilePhotoURL *string   `json:"profile_photo_url"`
	CreatedOn       time.Time `json:"created_on"`
}

type adminProviderDirectoryResponse struct {
	ID                         int                           `json:"id"`
	Role                       string                        `json:"role"`
	Name                       string                        `json:"name"`
	Surname                    string                        `json:"surname"`
	Email                      string                        `json:"email"`
	ProfilePhotoURL            *string                       `json:"profile_photo_url"`
	CreatedOn                  time.Time                     `json:"created_on"`
	Category                   adminProviderCategoryResponse `json:"category"`
	CoverageZones              []adminCoverageZoneResponse   `json:"coverage_zones"`
	IdentityVerificationStatus string                        `json:"identity_verification_status"`
	IdentityVerifiedOn         *time.Time                    `json:"identity_verified_on"`
}

type adminProviderCategoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type adminCoverageZoneResponse struct {
	ID           int    `json:"id"`
	MarketID     int    `json:"market_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	ParentZoneID *int   `json:"parent_zone_id"`
	Enabled      bool   `json:"enabled"`
}

func registerAdminListUsersSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existen los siguientes consumidores registrados:$`, suite.thereAreRegisteredConsumers)
	sc.Step(`^que no existen consumidores registrados$`, suite.thereAreNoRegisteredConsumers)
	sc.Step(`^que estoy autenticado como administrador "([^"]*)" con el permiso "([^"]*)"$`, suite.iAmAuthenticatedAsAdminWithPermission)
	sc.Step(`^consulto el directorio de consumidores$`, suite.requestConsumerDirectory)
	sc.Step(`^el directorio de consumidores contiene exactamente a:$`, suite.consumerDirectoryContainsExactly)
	sc.Step(`^el directorio de consumidores no incluye a "([^"]*)"$`, suite.consumerDirectoryDoesNotInclude)
	sc.Step(`^el consumidor "([^"]*)" incluye su identificador, rol, nombre, apellido, correo y fecha de registro$`, suite.consumerDirectoryIncludesAdministrativeData)
	sc.Step(`^el consumidor "([^"]*)" incluye la URL pública de su foto de perfil$`, suite.consumerDirectoryIncludesPublicProfilePhotoURL)
	sc.Step(`^el consumidor "([^"]*)" no expone credenciales ni identificadores del proveedor de identidad$`, suite.consumerDirectoryDoesNotExposeSensitiveData)
	sc.Step(`^el sistema devuelve un directorio de consumidores vacío$`, suite.systemReturnsEmptyConsumerDirectory)
	sc.Step(`^que existen los siguientes prestadores registrados en el rubro "([^"]*)":$`, suite.thereAreRegisteredProvidersInCategory)
	sc.Step(`^que no existen prestadores registrados$`, suite.thereAreNoRegisteredProviders)
	sc.Step(`^que la verificación de "([^"]*)" estuvo "([^"]*)" el "([^"]*)"$`, suite.providerVerificationWasAt)
	sc.Step(`^que la identidad de "([^"]*)" fue aprobada el "([^"]*)"$`, suite.providerIdentityWasApprovedOn)
	sc.Step(`^que "([^"]*)" no tiene sesiones de verificación de identidad$`, suite.noIdentityVerificationSessionForProvider)
	sc.Step(`^consulto el directorio de prestadores$`, suite.requestProviderDirectory)
	sc.Step(`^el directorio de prestadores contiene exactamente a:$`, suite.providerDirectoryContainsExactly)
	sc.Step(`^el directorio de prestadores no incluye a "([^"]*)"$`, suite.providerDirectoryDoesNotInclude)
	sc.Step(`^el prestador "([^"]*)" incluye el rubro "([^"]*)"$`, suite.providerDirectoryIncludesCategory)
	sc.Step(`^el prestador "([^"]*)" incluye exactamente las siguientes zonas de cobertura:$`, suite.providerDirectoryIncludesExactlyCoverageZones)
	sc.Step(`^cada zona incluida informa su identificador, market, código, nombre, tipo, zona padre y disponibilidad$`, suite.everyIncludedCoverageZoneHasCompleteData)
	sc.Step(`^el prestador "([^"]*)" incluye su identificador, rol, nombre, apellido, correo y fecha de registro$`, suite.providerDirectoryIncludesAdministrativeData)
	sc.Step(`^el prestador "([^"]*)" incluye la URL pública de su foto de perfil$`, suite.providerDirectoryIncludesPublicProfilePhotoURL)
	sc.Step(`^el prestador "([^"]*)" informa el estado de verificación "([^"]*)"$`, suite.providerDirectoryReportsVerificationStatus)
	sc.Step(`^el prestador "([^"]*)" informa la fecha de verificación "([^"]*)"$`, suite.providerDirectoryReportsVerificationDate)
	sc.Step(`^el prestador "([^"]*)" no informa una fecha de verificación$`, suite.providerDirectoryDoesNotReportVerificationDate)
	sc.Step(`^el prestador "([^"]*)" no expone credenciales, identificadores del proveedor de identidad, documentos ni códigos de riesgo$`, suite.providerDirectoryDoesNotExposeSensitiveData)
	sc.Step(`^el sistema devuelve un directorio de prestadores vacío$`, suite.systemReturnsEmptyProviderDirectory)
}

func (suite *testSuite) thereAreRegisteredConsumers(table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("consumer fixture table must contain a header and at least one consumer")
	}
	if err := requireTableHeaders(table, "correo", "nombre", "apellido"); err != nil {
		return err
	}

	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 3 {
			return fmt.Errorf("expected three consumer fixture columns, got %d", len(row.Cells))
		}
		if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(
			row.Cells[0].Value,
			row.Cells[1].Value,
			row.Cells[2].Value,
		); err != nil {
			return fmt.Errorf("registering consumer %q: %w", row.Cells[0].Value, err)
		}
	}

	return nil
}

func requireTableHeaders(table *godog.Table, expected ...string) error {
	if len(table.Rows) == 0 || len(table.Rows[0].Cells) != len(expected) {
		return fmt.Errorf("expected table headers %v", expected)
	}
	for index, header := range expected {
		if table.Rows[0].Cells[index].Value != header {
			return fmt.Errorf("expected table header %q at column %d, got %q", header, index+1, table.Rows[0].Cells[index].Value)
		}
	}
	return nil
}

func (suite *testSuite) thereAreNoRegisteredConsumers() error {
	return suite.userRepository.DeleteAllOf(consumer.Role)
}

func (suite *testSuite) iAmAuthenticatedAsAdminWithPermission(email, permission string) error {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return fmt.Errorf("administrator permission must not be empty")
	}

	suite.currentAuth0ID = auth0IDForAdminEmail(email)
	suite.currentPermissions = []string{permission}
	return nil
}

func (suite *testSuite) requestConsumerDirectory() error {
	return suite.requestAdminDirectory("/admin/consumers")
}

func (suite *testSuite) requestAdminDirectory(path string) error {
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+path, nil)
	if err != nil {
		return fmt.Errorf("creating administrative directory request: %w", err)
	}
	request.Header.Set(
		"Authorization",
		"Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions),
	)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("requesting administrative directory: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading administrative directory response: %w", err)
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	return nil
}

func (suite *testSuite) consumerDirectoryContainsExactly(table *godog.Table) error {
	if err := requireTableHeaders(table, "correo"); err != nil {
		return err
	}
	directory, _, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}

	expected := make(map[string]struct{}, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected one consumer assertion column, got %d", len(row.Cells))
		}
		expected[row.Cells[0].Value] = struct{}{}
	}
	if len(directory) != len(expected) {
		return fmt.Errorf("expected %d consumers, got %d with body %s", len(expected), len(directory), string(suite.lastBody))
	}
	for _, found := range directory {
		if _, ok := expected[found.Email]; !ok {
			return fmt.Errorf("consumer directory unexpectedly contains %q; body %s", found.Email, string(suite.lastBody))
		}
	}

	return nil
}

func (suite *testSuite) consumerDirectoryDoesNotInclude(email string) error {
	directory, _, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	for _, found := range directory {
		if found.Email == email {
			return fmt.Errorf("consumer directory unexpectedly includes %q", email)
		}
	}
	return nil
}

func (suite *testSuite) consumerDirectoryIncludesAdministrativeData(email string) error {
	directory, rawDirectory, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	found, raw, err := findConsumerDirectoryEntry(directory, rawDirectory, email)
	if err != nil {
		return err
	}
	if found.ID <= 0 || found.Role != consumer.Role || strings.TrimSpace(found.Name) == "" ||
		strings.TrimSpace(found.Surname) == "" || found.Email != email || found.CreatedOn.IsZero() {
		return fmt.Errorf("consumer %q has incomplete administrative data: %s", email, string(suite.lastBody))
	}

	expectedFields := map[string]struct{}{
		"id": {}, "role": {}, "name": {}, "surname": {}, "email": {}, "profile_photo_url": {}, "created_on": {},
	}
	if len(raw) != len(expectedFields) {
		return fmt.Errorf("consumer %q has unexpected response fields: %s", email, string(suite.lastBody))
	}
	for field := range expectedFields {
		if _, ok := raw[field]; !ok {
			return fmt.Errorf("consumer %q is missing response field %q", email, field)
		}
	}
	return nil
}

func (suite *testSuite) consumerDirectoryIncludesPublicProfilePhotoURL(email string) error {
	directory, rawDirectory, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	found, _, err := findConsumerDirectoryEntry(directory, rawDirectory, email)
	if err != nil {
		return err
	}
	if found.ProfilePhotoURL == nil {
		return fmt.Errorf("consumer %q has no profile photo URL", email)
	}
	parsed, err := url.Parse(*found.ProfilePhotoURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("consumer %q has invalid public profile photo URL %q", email, *found.ProfilePhotoURL)
	}
	return nil
}

func (suite *testSuite) consumerDirectoryDoesNotExposeSensitiveData(email string) error {
	directory, rawDirectory, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	_, raw, err := findConsumerDirectoryEntry(directory, rawDirectory, email)
	if err != nil {
		return err
	}
	for _, forbidden := range []string{"auth_id", "auth0_id", "password", "credentials", "permissions"} {
		if _, exposed := raw[forbidden]; exposed {
			return fmt.Errorf("consumer %q exposes sensitive field %q", email, forbidden)
		}
	}
	return nil
}

func (suite *testSuite) systemReturnsEmptyConsumerDirectory() error {
	directory, _, err := suite.consumerDirectoryResponse()
	if err != nil {
		return err
	}
	if directory == nil || len(directory) != 0 {
		return fmt.Errorf("expected JSON array [], got %s", string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) consumerDirectoryResponse() (
	[]adminConsumerDirectoryResponse,
	[]map[string]json.RawMessage,
	error,
) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, nil, err
	}

	var directory []adminConsumerDirectoryResponse
	if err := json.Unmarshal(suite.lastBody, &directory); err != nil {
		return nil, nil, fmt.Errorf("response is not a valid consumer directory: %w", err)
	}
	var rawDirectory []map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &rawDirectory); err != nil {
		return nil, nil, fmt.Errorf("response is not a valid consumer directory object list: %w", err)
	}
	return directory, rawDirectory, nil
}

func findConsumerDirectoryEntry(
	directory []adminConsumerDirectoryResponse,
	rawDirectory []map[string]json.RawMessage,
	email string,
) (adminConsumerDirectoryResponse, map[string]json.RawMessage, error) {
	for index, found := range directory {
		if found.Email == email {
			return found, rawDirectory[index], nil
		}
	}
	return adminConsumerDirectoryResponse{}, nil, fmt.Errorf("consumer directory does not include %q", email)
}

func (suite *testSuite) thereAreRegisteredProvidersInCategory(categoryName string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("provider fixture table must contain a header and at least one provider")
	}
	if err := requireTableHeaders(table, "correo", "nombre", "apellido"); err != nil {
		return err
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 3 {
			return fmt.Errorf("expected three provider fixture columns, got %d", len(row.Cells))
		}
		if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(
			row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value, categoryName,
		); err != nil {
			return fmt.Errorf("registering provider %q: %w", row.Cells[0].Value, err)
		}
	}
	return nil
}

func (suite *testSuite) thereAreNoRegisteredProviders() error {
	return suite.userRepository.DeleteAllOf(provider.Role)
}

func (suite *testSuite) providerVerificationWasAt(email, status, occurredOnText string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	occurredOn, err := time.Parse(time.RFC3339Nano, occurredOnText)
	if err != nil {
		return fmt.Errorf("parsing identity verification date %q: %w", occurredOnText, err)
	}
	verification, err := identityverification.NewVerification(
		providerID, uuid.New(), uuid.New(), "fake", 1, occurredOn.Add(-time.Minute),
	)
	if err != nil {
		return fmt.Errorf("creating identity verification fixture: %w", err)
	}
	if err := applyVerificationResult(verification, identityverification.VerificationStatus(status), occurredOn); err != nil {
		return err
	}
	if err := suite.identityVerificationRepository.Save(context.Background(), verification); err != nil {
		return fmt.Errorf("saving identity verification fixture: %w", err)
	}
	suite.expectedIdentityVerificationSessionID = verification.ExternalSessionID
	return nil
}

func (suite *testSuite) providerIdentityWasApprovedOn(email, occurredOnText string) error {
	providerID, err := suite.providerIDByEmail(email)
	if err != nil {
		return err
	}
	occurredOn, err := time.Parse(time.RFC3339Nano, occurredOnText)
	if err != nil {
		return fmt.Errorf("parsing identity approval date %q: %w", occurredOnText, err)
	}
	verification, err := suite.identityVerificationRepository.FindLatestByProviderID(context.Background(), providerID)
	if err != nil {
		return fmt.Errorf("finding latest identity verification for %q: %w", email, err)
	}
	if verification == nil {
		verification, err = identityverification.NewVerification(
			providerID, uuid.New(), uuid.New(), "fake", 1, occurredOn.Add(-time.Minute),
		)
		if err != nil {
			return fmt.Errorf("creating identity verification fixture: %w", err)
		}
	}
	if err := applyVerificationResult(verification, identityverification.StatusApproved, occurredOn); err != nil {
		return err
	}
	if err := suite.identityVerificationRepository.Save(context.Background(), verification); err != nil {
		return fmt.Errorf("saving approved identity verification fixture: %w", err)
	}
	suite.expectedIdentityVerificationSessionID = verification.ExternalSessionID
	return nil
}

func applyVerificationResult(
	verification *identityverification.IdentityVerification,
	status identityverification.VerificationStatus,
	occurredOn time.Time,
) error {
	err := verification.ApplyResult(identityverification.VerificationResult{
		EventID:         uuid.New(),
		SessionID:       verification.ExternalSessionID,
		ProviderID:      verification.ProviderID,
		VendorData:      identityverification.ProviderVendorData(verification.ProviderID),
		WorkflowID:      verification.WorkflowID,
		WorkflowVersion: verification.WorkflowVersion,
		Status:          status,
		OccurredOn:      occurredOn,
	}, occurredOn)
	if err != nil {
		return fmt.Errorf("applying identity verification status %q: %w", status, err)
	}
	return nil
}

func (suite *testSuite) requestProviderDirectory() error {
	return suite.requestAdminDirectory("/admin/providers")
}

func (suite *testSuite) providerDirectoryContainsExactly(table *godog.Table) error {
	if err := requireTableHeaders(table, "correo"); err != nil {
		return err
	}
	directory, _, err := suite.providerDirectoryResponse()
	if err != nil {
		return err
	}
	expected := make(map[string]struct{}, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected one provider assertion column, got %d", len(row.Cells))
		}
		expected[row.Cells[0].Value] = struct{}{}
	}
	if len(directory) != len(expected) {
		return fmt.Errorf("expected %d providers, got %d with body %s", len(expected), len(directory), suite.lastBody)
	}
	for _, found := range directory {
		if _, ok := expected[found.Email]; !ok {
			return fmt.Errorf("provider directory unexpectedly contains %q; body %s", found.Email, suite.lastBody)
		}
	}
	return nil
}

func (suite *testSuite) providerDirectoryDoesNotInclude(email string) error {
	directory, _, err := suite.providerDirectoryResponse()
	if err != nil {
		return err
	}
	for _, found := range directory {
		if found.Email == email {
			return fmt.Errorf("provider directory unexpectedly includes %q", email)
		}
	}
	return nil
}

func (suite *testSuite) providerDirectoryIncludesCategory(email, categoryName string) error {
	found, raw, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	if found.Category.ID <= 0 || found.Category.Name != categoryName {
		return fmt.Errorf("provider %q has unexpected category: %s", email, suite.lastBody)
	}
	var rawCategory map[string]json.RawMessage
	if err := json.Unmarshal(raw["category"], &rawCategory); err != nil {
		return fmt.Errorf("provider %q category is not an object: %w", email, err)
	}
	return requireExactJSONFields(rawCategory, "provider category", "id", "name")
}

func (suite *testSuite) providerDirectoryIncludesExactlyCoverageZones(email string, table *godog.Table) error {
	if err := requireTableHeaders(table, "zona"); err != nil {
		return err
	}
	found, _, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	expected := make(map[string]struct{}, len(table.Rows)-1)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != 1 {
			return fmt.Errorf("expected one coverage zone assertion column, got %d", len(row.Cells))
		}
		expected[row.Cells[0].Value] = struct{}{}
	}
	if len(found.CoverageZones) != len(expected) {
		return fmt.Errorf("expected %d coverage zones for provider %q, got %d: %s", len(expected), email, len(found.CoverageZones), suite.lastBody)
	}
	for _, zone := range found.CoverageZones {
		if _, ok := expected[zone.Name]; !ok {
			return fmt.Errorf("provider %q unexpectedly covers zone %q", email, zone.Name)
		}
	}
	return nil
}

func (suite *testSuite) everyIncludedCoverageZoneHasCompleteData() error {
	directory, rawDirectory, err := suite.providerDirectoryResponse()
	if err != nil {
		return err
	}
	zoneCount := 0
	for providerIndex, found := range directory {
		var rawZones []map[string]json.RawMessage
		if err := json.Unmarshal(rawDirectory[providerIndex]["coverage_zones"], &rawZones); err != nil {
			return fmt.Errorf("provider %q coverage_zones is not an object list: %w", found.Email, err)
		}
		if len(rawZones) != len(found.CoverageZones) {
			return fmt.Errorf("provider %q has an inconsistent coverage zone response", found.Email)
		}
		for zoneIndex, zone := range found.CoverageZones {
			zoneCount++
			if zone.ID <= 0 || zone.MarketID <= 0 || strings.TrimSpace(zone.Code) == "" ||
				strings.TrimSpace(zone.Name) == "" || strings.TrimSpace(zone.Kind) == "" {
				return fmt.Errorf("provider %q has an incomplete coverage zone: %s", found.Email, suite.lastBody)
			}
			if err := requireExactJSONFields(rawZones[zoneIndex], "coverage zone", "id", "market_id", "code", "name", "kind", "parent_zone_id", "enabled"); err != nil {
				return err
			}
		}
	}
	if zoneCount == 0 {
		return fmt.Errorf("expected at least one coverage zone, got none")
	}
	return nil
}

func (suite *testSuite) providerDirectoryIncludesAdministrativeData(email string) error {
	found, raw, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	if found.ID <= 0 || found.Role != provider.Role || strings.TrimSpace(found.Name) == "" ||
		strings.TrimSpace(found.Surname) == "" || found.Email != email || found.CreatedOn.IsZero() {
		return fmt.Errorf("provider %q has incomplete administrative data: %s", email, suite.lastBody)
	}
	return requireExactJSONFields(raw, "provider directory entry",
		"id", "role", "name", "surname", "email", "profile_photo_url", "created_on",
		"category", "coverage_zones", "identity_verification_status", "identity_verified_on",
	)
}

func (suite *testSuite) providerDirectoryIncludesPublicProfilePhotoURL(email string) error {
	found, _, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	if found.ProfilePhotoURL == nil {
		return fmt.Errorf("provider %q has no profile photo URL", email)
	}
	parsed, err := url.Parse(*found.ProfilePhotoURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("provider %q has invalid public profile photo URL %q", email, *found.ProfilePhotoURL)
	}
	return nil
}

func (suite *testSuite) providerDirectoryReportsVerificationStatus(email, expectedStatus string) error {
	found, _, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	if found.IdentityVerificationStatus != expectedStatus {
		return fmt.Errorf("expected provider %q verification status %q, got %q", email, expectedStatus, found.IdentityVerificationStatus)
	}
	return nil
}

func (suite *testSuite) providerDirectoryReportsVerificationDate(email, expectedDateText string) error {
	found, _, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	expectedDate, err := time.Parse(time.RFC3339Nano, expectedDateText)
	if err != nil {
		return fmt.Errorf("parsing expected identity verification date %q: %w", expectedDateText, err)
	}
	if found.IdentityVerifiedOn == nil || !found.IdentityVerifiedOn.Equal(expectedDate) {
		return fmt.Errorf("expected provider %q verification date %s, got %v", email, expectedDate.Format(time.RFC3339Nano), found.IdentityVerifiedOn)
	}
	return nil
}

func (suite *testSuite) providerDirectoryDoesNotReportVerificationDate(email string) error {
	found, _, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	if found.IdentityVerifiedOn != nil {
		return fmt.Errorf("expected provider %q not to report a verification date, got %s", email, found.IdentityVerifiedOn.Format(time.RFC3339Nano))
	}
	return nil
}

func (suite *testSuite) providerDirectoryDoesNotExposeSensitiveData(email string) error {
	_, raw, err := suite.findProviderDirectoryEntry(email)
	if err != nil {
		return err
	}
	for _, forbidden := range []string{
		"auth_id", "auth0_id", "password", "credentials", "permissions",
		"criminal_record_file", "cuit_certificate_file", "professional_credential",
		"biometric_validation_id", "risk_codes",
	} {
		if _, exposed := raw[forbidden]; exposed {
			return fmt.Errorf("provider %q exposes sensitive field %q", email, forbidden)
		}
	}
	return nil
}

func (suite *testSuite) systemReturnsEmptyProviderDirectory() error {
	directory, _, err := suite.providerDirectoryResponse()
	if err != nil {
		return err
	}
	if directory == nil || len(directory) != 0 {
		return fmt.Errorf("expected JSON array [], got %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) providerDirectoryResponse() (
	[]adminProviderDirectoryResponse,
	[]map[string]json.RawMessage,
	error,
) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, nil, err
	}
	var directory []adminProviderDirectoryResponse
	if err := json.Unmarshal(suite.lastBody, &directory); err != nil {
		return nil, nil, fmt.Errorf("response is not a valid provider directory: %w", err)
	}
	var rawDirectory []map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &rawDirectory); err != nil {
		return nil, nil, fmt.Errorf("response is not a valid provider directory object list: %w", err)
	}
	if len(directory) != len(rawDirectory) {
		return nil, nil, fmt.Errorf("provider directory decoding produced inconsistent entry counts")
	}
	return directory, rawDirectory, nil
}

func (suite *testSuite) findProviderDirectoryEntry(email string) (
	adminProviderDirectoryResponse,
	map[string]json.RawMessage,
	error,
) {
	directory, rawDirectory, err := suite.providerDirectoryResponse()
	if err != nil {
		return adminProviderDirectoryResponse{}, nil, err
	}
	for index, found := range directory {
		if found.Email == email {
			return found, rawDirectory[index], nil
		}
	}
	return adminProviderDirectoryResponse{}, nil, fmt.Errorf("provider directory does not include %q", email)
}

func requireExactJSONFields(raw map[string]json.RawMessage, objectName string, expected ...string) error {
	if len(raw) != len(expected) {
		return fmt.Errorf("%s has %d fields; expected exactly %d", objectName, len(raw), len(expected))
	}
	for _, field := range expected {
		if _, ok := raw[field]; !ok {
			return fmt.Errorf("%s is missing response field %q", objectName, field)
		}
	}
	return nil
}
