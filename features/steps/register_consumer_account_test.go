package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

const registerConsumerEndpoint = "http://localhost:8080/consumers"

type consumerRegistrationRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type registrationResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func existeUnConsumidorRegistradoConCorreo(correo string) error {
	req := consumerRegistrationRequest{
		Email:    correo,
		Name:     "Consumidor Existente",
		Password: "Segura12345?",
	}

	resp, err := postConsumerRegistration(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("no se pudo preparar el consumidor existente: codigo %d, cuerpo %s", resp.StatusCode, string(body))
	}

	return nil
}

func noExisteUnConsumidorConCorreo(_ string) error {
	return nil
}

func solicitoRegistrarUnaCuentaDeConsumidor(ctx *testContext, correo, nombre, contrasena string) error {
	resp, err := postConsumerRegistration(consumerRegistrationRequest{
		Email:    correo,
		Name:     nombre,
		Password: contrasena,
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	ctx.responseBody = string(body)
	ctx.statusCode = resp.StatusCode

	return nil
}

func elSistemaConfirmaElRegistro(ctx *testContext) error {
	if err := laRespuestaDeRegistroDebeTenerUnCodigo(ctx, http.StatusCreated); err != nil {
		return err
	}

	return laRespuestaDeRegistroDebeIndicar(ctx, "cuenta registrada exitosamente")
}

func elSistemaMeIndicaQueElFormatoDelCorreoEsInvalido(ctx *testContext) error {
	if err := laRespuestaDeRegistroDebeTenerUnCodigo(ctx, http.StatusBadRequest); err != nil {
		return err
	}

	return laRespuestaDeRegistroDebeIndicar(ctx, "correo electronico invalido")
}

func elSistemaMeIndicaQueLaContrasenaEsDemasiadoCorta(ctx *testContext) error {
	if err := laRespuestaDeRegistroDebeTenerUnCodigo(ctx, http.StatusBadRequest); err != nil {
		return err
	}

	return laRespuestaDeRegistroDebeIndicar(ctx, "contraseña demasiado corta")
}

func elSistemaMeIndicaQueLaContrasenaEsInsegura(ctx *testContext) error {
	if err := laRespuestaDeRegistroDebeTenerUnCodigo(ctx, http.StatusBadRequest); err != nil {
		return err
	}

	return laRespuestaDeRegistroDebeIndicar(ctx, "contraseña insegura")
}

func elSistemaMeIndicaQueElCorreoYaEstaRegistrado(ctx *testContext) error {
	if err := laRespuestaDeRegistroDebeTenerUnCodigo(ctx, http.StatusConflict); err != nil {
		return err
	}

	return laRespuestaDeRegistroDebeIndicar(ctx, "correo electronico ya registrado")
}

func laRespuestaDeRegistroDebeTenerUnCodigo(ctx *testContext, codigo int) error {
	if ctx.statusCode != codigo {
		return fmt.Errorf("se esperaba codigo %d, pero se obtuvo %d con cuerpo %s", codigo, ctx.statusCode, ctx.responseBody)
	}
	return nil
}

func laRespuestaDeRegistroDebeIndicar(ctx *testContext, mensaje string) error {
	var response registrationResponse
	if err := json.Unmarshal([]byte(ctx.responseBody), &response); err != nil {
		return fmt.Errorf("la respuesta no es JSON valido: %w", err)
	}

	if response.Message == mensaje || response.Error == mensaje {
		return nil
	}

	return fmt.Errorf("se esperaba mensaje %q, pero se obtuvo cuerpo %s", mensaje, ctx.responseBody)
}

func postConsumerRegistration(req consumerRegistrationRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(registerConsumerEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fallo la conexion a la API: %w", err)
	}

	return resp, nil
}

func registrarPasosDeCuentaConsumidor(ctx *godog.ScenarioContext, tCtx *testContext) {
	ctx.Step(`^que no existe un consumidor con correo "([^"]*)"$`, noExisteUnConsumidorConCorreo)
	ctx.Step(`^existe un consumidor registrado con correo "([^"]*)"$`, existeUnConsumidorRegistradoConCorreo)
	ctx.Step(`^me registro como usuario consumidor con correo "([^"]*)", nombre "([^"]*)" y contraseña "([^"]*)"$`, func(correo, nombre, contrasena string) error {
		return solicitoRegistrarUnaCuentaDeConsumidor(tCtx, correo, nombre, contrasena)
	})
	ctx.Step(`^solicito registrar una cuenta de consumidor con correo "([^"]*)", nombre "([^"]*)" y contraseña "([^"]*)"$`, func(correo, nombre, contrasena string) error {
		return solicitoRegistrarUnaCuentaDeConsumidor(tCtx, correo, nombre, contrasena)
	})
	ctx.Step(`^el sistema confirma el registro$`, func() error {
		return elSistemaConfirmaElRegistro(tCtx)
	})
	ctx.Step(`^el sistema me indica que el formato del correo es inválido$`, func() error {
		return elSistemaMeIndicaQueElFormatoDelCorreoEsInvalido(tCtx)
	})
	ctx.Step(`^el sistema me indica que la contraseña es demasiado corta$`, func() error {
		return elSistemaMeIndicaQueLaContrasenaEsDemasiadoCorta(tCtx)
	})
	ctx.Step(`^el sistema me indica que la contraseña es insegura$`, func() error {
		return elSistemaMeIndicaQueLaContrasenaEsInsegura(tCtx)
	})
	ctx.Step(`^el sistema me indica que el correo electrónico ya está registrado$`, func() error {
		return elSistemaMeIndicaQueElCorreoYaEstaRegistrado(tCtx)
	})
	ctx.Step(`^la respuesta de registro debe tener un codigo (\d+)$`, func(codigo int) error {
		return laRespuestaDeRegistroDebeTenerUnCodigo(tCtx, codigo)
	})
	ctx.Step(`^la respuesta de registro debe indicar "([^"]*)"$`, func(mensaje string) error {
		return laRespuestaDeRegistroDebeIndicar(tCtx, mensaje)
	})
}
