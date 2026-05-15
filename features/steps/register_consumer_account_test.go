package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

type consumerRegistrationRequest struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

type registrationResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func registerConsumerAccountSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existe un consumidor con correo "([^"]*)"$`, suite.noExisteUnConsumidorConCorreo)
	sc.Step(`^existe un consumidor registrado con correo "([^"]*)"$`, suite.existeUnConsumidorRegistradoConCorreo)
	sc.Step(`^me registro como usuario consumidor con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)" y contraseña "([^"]*)"$`, suite.solicitoRegistrarUnaCuentaDeConsumidor)
	sc.Step(`^el sistema confirma el registro$`, suite.elSistemaConfirmaElRegistro)
	sc.Step(`^el sistema me indica que el formato del correo es inválido$`, suite.elSistemaMeIndicaQueElFormatoDelCorreoEsInvalido)
	sc.Step(`^el sistema me indica que la contraseña es demasiado corta$`, suite.elSistemaMeIndicaQueLaContrasenaEsDemasiadoCorta)
	sc.Step(`^el sistema me indica que la contraseña es insegura$`, suite.elSistemaMeIndicaQueLaContrasenaEsInsegura)
	sc.Step(`^el sistema me indica que el correo electrónico ya está registrado$`, suite.elSistemaMeIndicaQueElCorreoYaEstaRegistrado)
	sc.Step(`^la respuesta de registro debe tener un codigo (\d+)$`, suite.laRespuestaDeRegistroDebeTenerUnCodigo)
	sc.Step(`^la respuesta de registro debe indicar "([^"]*)"$`, suite.laRespuestaDeRegistroDebeIndicar)
}

func (suite *testSuite) existeUnConsumidorRegistradoConCorreo(correo string) error {
	req := consumerRegistrationRequest{
		Email: correo,
		Name:  "Consumidor Existente",
	}

	resp, err := suite.postConsumerRegistration(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("no se pudo preparar el consumidor existente: codigo %d, cuerpo %s", resp.StatusCode, string(body))
	}

	return nil
}

func (suite *testSuite) noExisteUnConsumidorConCorreo(_ string) error {
	return suite.consumerRepository.DeleteAll()
}

func (suite *testSuite) solicitoRegistrarUnaCuentaDeConsumidor(correo, nombre, surname, _ string) error {
	resp, err := suite.postConsumerRegistration(consumerRegistrationRequest{
		Email:   correo,
		Name:    nombre,
		Surname: surname,
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

func (suite *testSuite) elSistemaConfirmaElRegistro() error {
	if err := suite.laRespuestaDeRegistroDebeTenerUnCodigo(http.StatusCreated); err != nil {
		return err
	}

	return suite.laRespuestaDeRegistroDebeIndicar("cuenta registrada exitosamente")
}

func (suite *testSuite) elSistemaMeIndicaQueElFormatoDelCorreoEsInvalido() error {
	if err := suite.laRespuestaDeRegistroDebeTenerUnCodigo(http.StatusBadRequest); err != nil {
		return err
	}

	return suite.laRespuestaDeRegistroDebeIndicar("correo electronico invalido")
}

func (suite *testSuite) elSistemaMeIndicaQueLaContrasenaEsDemasiadoCorta() error {
	if err := suite.laRespuestaDeRegistroDebeTenerUnCodigo(http.StatusBadRequest); err != nil {
		return err
	}

	return suite.laRespuestaDeRegistroDebeIndicar("contraseña demasiado corta")
}

func (suite *testSuite) elSistemaMeIndicaQueLaContrasenaEsInsegura() error {
	if err := suite.laRespuestaDeRegistroDebeTenerUnCodigo(http.StatusBadRequest); err != nil {
		return err
	}

	return suite.laRespuestaDeRegistroDebeIndicar("contraseña insegura")
}

func (suite *testSuite) elSistemaMeIndicaQueElCorreoYaEstaRegistrado() error {
	if err := suite.laRespuestaDeRegistroDebeTenerUnCodigo(http.StatusConflict); err != nil {
		return err
	}

	return suite.laRespuestaDeRegistroDebeIndicar("correo electronico ya registrado")
}

func (suite *testSuite) laRespuestaDeRegistroDebeTenerUnCodigo(codigo int) error {
	if suite.lastStatus != codigo {
		return fmt.Errorf("se esperaba codigo %d, pero se obtuvo %d con cuerpo %s", codigo, suite.lastStatus, string(suite.lastBody))
	}
	return nil
}

func (suite *testSuite) laRespuestaDeRegistroDebeIndicar(mensaje string) error {
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("la respuesta no es JSON valido: %w", err)
	}

	if response.Message == mensaje || response.Error == mensaje {
		return nil
	}

	return fmt.Errorf("se esperaba mensaje %q, pero se obtuvo cuerpo %s", mensaje, string(suite.lastBody))
}

func (suite *testSuite) postConsumerRegistration(req consumerRegistrationRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/consumers", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken("auth0|consumer-test", nil))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fallo la conexion a la API: %w", err)
	}

	return resp, nil
}
