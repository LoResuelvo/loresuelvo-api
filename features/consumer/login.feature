Feature: Iniciar sesión

    Como usuario registrado
    Quiero iniciar sesión en la plataforma
    Para acceder a mis servicios y funcionalidades personalizadas

    Scenario: 01-ME Obtener sesión de consumidor autenticado
        Given que estoy registrado como consumidor
        When consulto mi información de usuario autenticado
        Then el sistema informa que tengo rol "consumidor"

    Scenario: 02-ME Obtener sesión de prestador autenticado
        Given que estoy registrado como prestador
        When consulto mi información de usuario autenticado
        Then el sistema informa que tengo rol "prestador"

    Scenario: 03-ME Rechazar usuario sin sesión válida
        Given que no tengo una sesión válida
        When consulto mi información de usuario autenticado
        Then el sistema deniega el acceso
