package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

type providerRegistrationRequest struct {
	Email                  string `json:"email"`
	Name                   string `json:"name"`
	Surname                string `json:"surname"`
	Category               string `json:"category"`
	CoverageZone           string `json:"coverage_zone"`
	CriminalRecordFile     string `json:"criminal_record_file"`
	CUITCertificateFile    string `json:"cuit_certificate_file"`
	BiometricValidationID  string `json:"biometric_validation_id"`
	ProfessionalCredential string `json:"professional_credential_file"`
}

func registerProviderAccountSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existe un usuario con correo "([^"]*)"$`, suite.noExisteUnUsuarioConCorreo)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)", zona de cobertura "([^"]*)" e ingreso mis documentos obligatorios$`, suite.solicitoRegistrarUnaCuentaDePrestador)
}

func (suite *testSuite) noExisteUnUsuarioConCorreo(_ string) error {
	return suite.consumerRepository.DeleteAll()
}

func (suite *testSuite) solicitoRegistrarUnaCuentaDePrestador(correo, nombre, apellido, rubro, zonaDeCobertura string) error {
	resp, err := suite.postProviderRegistration(providerRegistrationRequest{
		Email:                  correo,
		Name:                   nombre,
		Surname:                apellido,
		Category:               rubro,
		CoverageZone:           zonaDeCobertura,
		CriminalRecordFile:     "antecedentes-penales.pdf",
		CUITCertificateFile:    "constancia-cuit.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "matricula-o-certificacion.pdf",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("fallo leyendo el cuerpo de la respuesta: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body

	return nil
}

func (suite *testSuite) postProviderRegistration(req providerRegistrationRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/providers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken("auth0|provider-test", nil))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fallo la conexion a la API: %w", err)
	}

	return resp, nil
}
