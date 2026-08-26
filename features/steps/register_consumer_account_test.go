package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/cucumber/godog"
)

type consumerRegistrationRequest struct {
	Email              string                       `json:"email"`
	Name               string                       `json:"name"`
	Surname            string                       `json:"surname"`
	ProfilePhotoFileID string                       `json:"profile_photo_file_id,omitempty"`
	Address            *consumerRegistrationAddress `json:"address,omitempty"`
	omitAddress        bool
}

type consumerRegistrationAddress struct {
	Street       string `json:"street"`
	StreetNumber string `json:"street_number"`
	Floor        string `json:"floor,omitempty"`
	Unit         string `json:"unit,omitempty"`
}

type registrationResponse struct {
	Message         string `json:"message"`
	Error           string `json:"error"`
	ProfilePhotoURL string `json:"profile_photo_url"`
}

func registerConsumerAccountSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^existe un consumidor registrado con correo "([^"]*)"$`, suite.thereIsRegisteredConsumerWithEmail)
	sc.Step(`^que cargué una foto de perfil válida para mi registro como consumidor$`, suite.uploadValidConsumerProfilePhoto)
	sc.Step(`^me registro como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)"$`, suite.requestConsumerAccountRegistration)
	sc.Step(`^me registro como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" sin cargar foto de perfil$`, suite.requestConsumerAccountRegistrationWithoutProfilePhoto)
	sc.Step(`^me registro como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" indicando como domicilio "([^"]*)"$`, suite.requestConsumerAccountRegistrationWithAddress)
	sc.Step(`^me registro como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" indicando como domicilio "([^"]*)", piso "([^"]*)" y departamento "([^"]*)"$`, suite.requestConsumerAccountRegistrationWithAddressDetails)
	sc.Step(`^intento registrarme como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" sin indicar un domicilio$`, suite.requestConsumerAccountRegistrationWithoutAddress)
	sc.Step(`^intento registrarme como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" indicando solamente el número "([^"]*)"$`, suite.requestConsumerAccountRegistrationWithOnlyNumber)
	sc.Step(`^intento registrarme como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" indicando solamente la calle "([^"]*)"$`, suite.requestConsumerAccountRegistrationWithOnlyStreet)
	sc.Step(`^intento registrarme como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" indicando un domicilio inexistente$`, suite.requestConsumerAccountRegistrationWithInvalidAddress)
	sc.Step(`^intento registrarme como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)" indicando el domicilio "([^"]*)"$`, suite.requestConsumerAccountRegistrationWithAddress)
	sc.Step(`^que el servicio de resolución de ubicación no está disponible$`, suite.locationResolutionServiceIsUnavailable)
	sc.Step(`^intento registrarme como consumidor utilizando una foto de perfil no disponible$`, suite.tryConsumerRegistrationWithUnavailableProfilePhoto)
	sc.Step(`^el sistema confirma el registro$`, suite.systemConfirmsRegistration)
	sc.Step(`^el registro del consumidor incluye su foto de perfil$`, suite.consumerRegistrationIncludesProfilePhoto)
	sc.Step(`^el registro del consumidor no incluye una foto de perfil$`, suite.consumerRegistrationDoesNotIncludeProfilePhoto)
	sc.Step(`^el sistema me indica que el formato del correo es inválido$`, suite.systemReportsInvalidEmailFormat)
	sc.Step(`^el sistema me indica que el correo electrónico ya está registrado$`, suite.systemReportsEmailAlreadyRegistered)
	sc.Step(`^la respuesta de registro debe tener un codigo (\d+)$`, suite.registrationResponseShouldHaveStatusCode)
	sc.Step(`^la respuesta de registro debe indicar "([^"]*)"$`, suite.registrationResponseShouldSay)
	sc.Step(`^el domicilio del consumidor conserva la calle "([^"]*)" y el número "([^"]*)"$`, suite.consumerAddressPreservesStreetAndNumber)
	sc.Step(`^el domicilio del consumidor conserva el piso "([^"]*)" y el departamento "([^"]*)"$`, suite.consumerAddressPreservesFloorAndUnit)
	sc.Step(`^el domicilio del consumidor tiene coordenadas válidas$`, suite.consumerAddressHasValidCoordinates)
	sc.Step(`^el domicilio del consumidor queda asociado a la zona de cobertura "([^"]*)"$`, suite.consumerAddressHasCoverageZone)
	sc.Step(`^el sistema me indica que la dirección es obligatoria$`, suite.systemReportsAddressRequired)
	sc.Step(`^el sistema me indica que la calle es obligatoria$`, suite.systemReportsStreetRequired)
	sc.Step(`^el sistema me indica que el número es obligatorio$`, suite.systemReportsStreetNumberRequired)
	sc.Step(`^el sistema me indica que no pudo validar la dirección$`, suite.systemReportsAddressNotValidated)
	sc.Step(`^el sistema me indica que todavía no ofrece servicios en esa ubicación$`, suite.systemReportsConsumerCoverageZoneUnavailable)
	sc.Step(`^el sistema me indica que no puede validar la dirección temporalmente$`, suite.systemReportsLocationServiceUnavailable)
}

func (suite *testSuite) thereIsRegisteredConsumerWithEmail(email string) error {
	req := consumerRegistrationRequest{
		Email: email,
		Name:  "Existing Consumer",
	}

	resp, err := suite.postConsumerRegistration(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("could not prepare existing consumer: status %d, body %s", resp.StatusCode, string(body))
	}

	return nil
}

func (suite *testSuite) requestConsumerAccountRegistration(email, name, surname string) error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email:              email,
		Name:               name,
		Surname:            surname,
		ProfilePhotoFileID: suite.consumerProfilePhotoFileID,
	})
}

func (suite *testSuite) requestConsumerAccountRegistrationWithoutProfilePhoto(email, name, surname string) error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email:   email,
		Name:    name,
		Surname: surname,
	})
}

func (suite *testSuite) requestConsumerAccountRegistrationWithAddress(email, name, surname, description string) error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email:   email,
		Name:    name,
		Surname: surname,
		Address: parseConsumerAddress(description),
	})
}

func (suite *testSuite) requestConsumerAccountRegistrationWithAddressDetails(email, name, surname, description, floor, unit string) error {
	address := parseConsumerAddress(description)
	address.Floor = floor
	address.Unit = unit
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email:   email,
		Name:    name,
		Surname: surname,
		Address: address,
	})
}

func (suite *testSuite) requestConsumerAccountRegistrationWithoutAddress(email, name, surname string) error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email:       email,
		Name:        name,
		Surname:     surname,
		omitAddress: true,
	})
}

func (suite *testSuite) requestConsumerAccountRegistrationWithOnlyNumber(email, name, surname, number string) error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email: email, Name: name, Surname: surname,
		Address: &consumerRegistrationAddress{StreetNumber: number},
	})
}

func (suite *testSuite) requestConsumerAccountRegistrationWithOnlyStreet(email, name, surname, street string) error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email: email, Name: name, Surname: surname,
		Address: &consumerRegistrationAddress{Street: street},
	})
}

func (suite *testSuite) requestConsumerAccountRegistrationWithInvalidAddress(email, name, surname string) error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email: email, Name: name, Surname: surname,
		Address: &consumerRegistrationAddress{Street: "Domicilio inexistente", StreetNumber: "1"},
	})
}

func (suite *testSuite) locationResolutionServiceIsUnavailable() error {
	controller, ok := suite.consumerAddressResolver.(interface{ SetAvailable(bool) })
	if !ok {
		return fmt.Errorf("test address resolver does not expose availability control")
	}
	controller.SetAvailable(false)
	return nil
}

func (suite *testSuite) tryConsumerRegistrationWithUnavailableProfilePhoto() error {
	return suite.performConsumerAccountRegistration(consumerRegistrationRequest{
		Email:              "ana@example.com",
		Name:               "Ana Perez",
		Surname:            "Mamani Tipula",
		ProfilePhotoFileID: "00000000-0000-0000-0000-000000000000",
	})
}

func (suite *testSuite) performConsumerAccountRegistration(request consumerRegistrationRequest) error {
	resp, err := suite.postConsumerRegistration(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body

	return nil
}

func (suite *testSuite) uploadValidConsumerProfilePhoto() error {
	fileID, err := suite.uploadValidProfilePhotoFor("auth0|consumer-test")
	if err != nil {
		return err
	}

	suite.consumerProfilePhotoFileID = fileID
	return nil
}

func (suite *testSuite) consumerRegistrationIncludesProfilePhoto() error {
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("consumer registration response is not valid JSON: %w", err)
	}
	if response.ProfilePhotoURL == "" {
		return fmt.Errorf("expected consumer registration response to include profile_photo_url, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) consumerRegistrationDoesNotIncludeProfilePhoto() error {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("consumer registration response is not valid JSON: %w", err)
	}
	if _, exists := response["profile_photo_url"]; exists {
		return fmt.Errorf("expected consumer registration response not to include profile_photo_url, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemConfirmsRegistration() error {
	return suite.registrationResponseShouldHaveStatusCode(http.StatusCreated)
}

func (suite *testSuite) systemReportsInvalidEmailFormat() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}

	return nil
}

func (suite *testSuite) systemReportsEmailAlreadyRegistered() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusConflict); err != nil {
		return err
	}

	return nil
}

func (suite *testSuite) registrationResponseShouldHaveStatusCode(statusCode int) error {
	if suite.lastStatus != statusCode {
		return fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) registrationResponseShouldSay(message string) error {
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}

	if response.Message == message || response.Error == message {
		return nil
	}

	return fmt.Errorf("expected message %q, got body %s", message, string(suite.lastBody))
}

func (suite *testSuite) consumerAddressPreservesStreetAndNumber(street, number string) error {
	registeredConsumer, err := suite.registeredConsumer()
	if err != nil {
		return err
	}
	if registeredConsumer.Address().Street != street || registeredConsumer.Address().StreetNumber != number {
		return fmt.Errorf("expected address %q %q, got %q %q", street, number, registeredConsumer.Address().Street, registeredConsumer.Address().StreetNumber)
	}
	return nil
}

func (suite *testSuite) consumerAddressPreservesFloorAndUnit(floor, unit string) error {
	registeredConsumer, err := suite.registeredConsumer()
	if err != nil {
		return err
	}
	if registeredConsumer.Address().Floor != floor || registeredConsumer.Address().Unit != unit {
		return fmt.Errorf("expected floor/unit %q/%q, got %q/%q", floor, unit, registeredConsumer.Address().Floor, registeredConsumer.Address().Unit)
	}
	return nil
}

func (suite *testSuite) consumerAddressHasValidCoordinates() error {
	registeredConsumer, err := suite.registeredConsumer()
	if err != nil {
		return err
	}
	return registeredConsumer.Location().Validate()
}

func (suite *testSuite) consumerAddressHasCoverageZone(zoneName string) error {
	registeredConsumer, err := suite.registeredConsumer()
	if err != nil {
		return err
	}
	zone, err := suite.coverageZoneRepository.FindByID(context.Background(), registeredConsumer.CoverageZone().ID)
	if err != nil {
		return err
	}
	if zone.Name != zoneName {
		return fmt.Errorf("expected coverage zone %q, got %q", zoneName, zone.Name)
	}
	return nil
}

func (suite *testSuite) registeredConsumer() (*consumer.Consumer, error) {
	consumerID, err := suite.userRepository.FindIDByEmail("ana@example.com")
	if err != nil {
		return nil, fmt.Errorf("finding registered consumer id: %w", err)
	}
	foundConsumer, err := suite.userRepository.FindConsumerByID(consumerID)
	if err != nil {
		return nil, fmt.Errorf("finding registered consumer: %w", err)
	}
	return foundConsumer, nil
}

func (suite *testSuite) systemReportsAddressRequired() error {
	return suite.registrationResponseShouldSay(consumer.ErrAddressRequired.Error())
}

func (suite *testSuite) systemReportsStreetRequired() error {
	return suite.registrationResponseShouldSay(consumer.ErrStreetRequired.Error())
}

func (suite *testSuite) systemReportsStreetNumberRequired() error {
	return suite.registrationResponseShouldSay(consumer.ErrStreetNumberRequired.Error())
}

func (suite *testSuite) systemReportsAddressNotValidated() error {
	return suite.registrationResponseShouldSay(consumer.ErrAddressNotValidated.Error())
}

func (suite *testSuite) systemReportsConsumerCoverageZoneUnavailable() error {
	return suite.registrationResponseShouldSay(consumer.ErrCoverageZoneNotAvailable.Error())
}

func (suite *testSuite) systemReportsLocationServiceUnavailable() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusServiceUnavailable); err != nil {
		return err
	}
	return suite.registrationResponseShouldSay(consumer.ErrAddressServiceUnavailable.Error())
}

func (suite *testSuite) postConsumerRegistration(req consumerRegistrationRequest) (*http.Response, error) {
	return suite.postConsumerRegistrationWithAuth0ID("auth0|consumer-test", req)
}

func (suite *testSuite) postConsumerRegistrationWithAuth0ID(auth0ID string, req consumerRegistrationRequest) (*http.Response, error) {
	if req.Address == nil && !req.omitAddress {
		req.Address = &consumerRegistrationAddress{Street: "Av. Rivadavia", StreetNumber: "5100"}
	}
	req.omitAddress = false
	if req.Address != nil {
		if _, err := suite.ensureDefaultProviderCoverageZone(); err != nil {
			return nil, fmt.Errorf("preparing consumer coverage zone: %w", err)
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/consumers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(auth0ID, nil))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API connection failed: %w", err)
	}

	return resp, nil
}

func parseConsumerAddress(description string) *consumerRegistrationAddress {
	parts := strings.SplitN(strings.TrimSpace(description), ",", 2)
	firstPart := strings.TrimSpace(parts[0])
	streetSuffix := ""
	if len(parts) == 2 {
		streetSuffix = strings.TrimSpace(parts[1])
	}

	fields := strings.Fields(firstPart)
	address := &consumerRegistrationAddress{}
	if len(fields) > 0 && allDigits(fields[len(fields)-1]) {
		address.StreetNumber = fields[len(fields)-1]
		address.Street = strings.Join(fields[:len(fields)-1], " ")
	} else {
		address.Street = firstPart
	}
	if streetSuffix != "" {
		address.Street = strings.TrimSpace(address.Street + ", " + streetSuffix)
	}
	return address
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}
