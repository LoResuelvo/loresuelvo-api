@wip
Feature: Enviar solicitud a prestador
    Como consumidor
    quiero enviar un mensaje de solicitud de contacto a un prestador con el que nunca interactué
    para proponerle un trabajo e iniciar un chat

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que no existe una conversación entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"

    Rule: Un consumidor autenticado puede iniciar una solicitud de contacto con un prestador

    Scenario: 01-ESP Enviar una solicitud de contacto correctamente
        Given que estoy autenticado como consumidor "ana@example.com"
        When envío una solicitud de trabajo al prestador "juan.plomero@example.com" con el mensaje:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        Then el sistema crea una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And la conversación contiene el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Rule: La solicitud debe identificar un prestador válido

    Scenario: 02-ESP Rechazar solicitud a un prestador inexistente
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento enviar una solicitud de trabajo a un prestador inexistente con el mensaje "Hola, necesito un presupuesto"
        Then el sistema me indica que el prestador no existe

    Rule: La solicitud debe incluir un mensaje inicial válido

    Scenario: 03-ESP Rechazar solicitud sin mensaje inicial
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento enviar una solicitud de trabajo al prestador "juan.plomero@example.com" sin mensaje inicial
        Then el sistema me indica que el mensaje es obligatorio

    Scenario: 04-ESP Rechazar solicitud con mensaje inicial vacío
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento enviar una solicitud de trabajo al prestador "juan.plomero@example.com" con el mensaje "   "
        Then el sistema me indica que el mensaje es obligatorio

    Rule: Solo un consumidor autenticado puede iniciar una solicitud de trabajo

    Scenario: 05-ESP Rechazar solicitud sin sesión válida
        Given que no tengo una sesión válida
        When intento enviar una solicitud de trabajo al prestador "juan.plomero@example.com" con el mensaje "Hola, necesito un presupuesto"
        Then el sistema deniega el acceso

    Scenario: 06-ESP Rechazar solicitud iniciada por un prestador
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una solicitud de trabajo al prestador "juan.plomero@example.com" con el mensaje "Hola, necesito un presupuesto"
        Then el sistema me indica que solo un consumidor puede iniciar una solicitud de trabajo

    Rule: No se deben duplicar conversaciones entre el mismo consumidor y prestador

    Scenario: 07-ESP Rechazar una nueva solicitud cuando ya existe una conversación pendiente
        Given que estoy autenticado como consumidor "ana@example.com"
        And que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        When intento enviar una solicitud de trabajo al prestador "juan.plomero@example.com" con el mensaje "Quería agregarte más detalles del trabajo"
        Then el sistema me indica que ya existe una conversación con ese prestador
