Feature: Crear rubro de prestador
    Como administrador
    quiero crear rubros técnicos
    para que los prestadores puedan registrarse con un rubro definido

    Background:
        # Given que soy administrador con sesión válida
        Given que no existe el rubro "Plomería"

    Scenario: 01-CR Crear un rubro correctamente
        When creo el rubro "Plomería"
        Then el sistema confirma la creación del rubro

    Rule: El rubro debe tener un nombre válido

    Scenario: 02-CR Rechazar creación sin nombre
        When intento crear un rubro sin nombre
        Then el sistema me indica que el nombre del rubro es obligatorio

    Scenario: 03-CR Rechazar creación con nombre vacío
        When intento crear el rubro "   "
        Then el sistema me indica que el nombre del rubro es obligatorio

    Rule: No se puede crear un rubro duplicado

    Scenario: 04-CR Rechazar creación de un rubro con nombre ya existente
        Given existe el rubro "Plomería"
        When intento crear el rubro "Plomería"
        Then el sistema me indica que el rubro ya existe

    Scenario: 05-CR Rechazar creación de un rubro duplicado con diferencias de mayúsculas o espacios
        Given existe el rubro "Plomería"
        When intento crear el rubro "  plomería  "
        Then el sistema me indica que el rubro ya existe

    @wip
    Scenario: 06-CR Crear un rubro que requiere matrícula profesional
        When creo el rubro "Gasista matriculado" que requiere matrícula profesional
        Then el sistema confirma la creación del rubro
        And el rubro "Gasista matriculado" queda disponible para registrar prestadores
        And el sistema informa que el rubro requiere matrícula profesional


    Rule: Solo usuarios autorizados pueden crear rubros

    @wip
    Scenario: 07-CR Rechazar creación de rubro sin sesión válida
        Given que no tengo una sesión válida
        When intento crear el rubro "Plomería" que no requiere matrícula profesional
        Then el sistema deniega el acceso
