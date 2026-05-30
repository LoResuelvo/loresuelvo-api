Feature: Obtener conversación
    Como usuario
    quiero consultar el detalle de una conversación existente
    para ver el estado de la solicitud y los mensajes intercambiados

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Rule: Los participantes de la conversación pueden consultar su detalle

    Scenario: 01-OC Consumidor obtiene una conversación existente
        Given que estoy autenticado como consumidor "ana@example.com"
        When consulto la conversación pendiente con el prestador "juan.plomero@example.com"
        Then el sistema muestra la conversación entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And el detalle de la conversación incluye el mensaje inicial enviado por "ana@example.com":
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Scenario: 02-OC Prestador obtiene una conversación existente
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When consulto la conversación pendiente con el consumidor "ana@example.com"
        Then el sistema muestra la conversación entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And el detalle de la conversación incluye el mensaje inicial enviado por "ana@example.com":
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Rule: Solo los participantes pueden consultar una conversación

    Scenario: 03-OC Rechazar consulta de consumidor que no participa en la conversación
        Given que estoy autenticado como consumidor "carla@example.com"
        When intento consultar la conversación pendiente con el prestador "juan.plomero@example.com"
        Then el sistema me indica que no puedo acceder a esa conversación

    Scenario: 04-OC Rechazar consulta de prestador que no participa en la conversación
        Given que estoy autenticado como prestador "pedro.plomero@example.com"
        When intento consultar la conversación pendiente con el consumidor "ana@example.com"
        Then el sistema me indica que no puedo acceder a esa conversación

    Scenario: 05-OC Rechazar consulta sin sesión válida
        Given que no tengo una sesión válida
        When intento consultar la conversación pendiente con el prestador "juan.plomero@example.com"
        Then el sistema deniega el acceso

    Rule: La conversación solicitada debe existir

    Scenario: 06-OC Rechazar consulta de conversación inexistente
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento consultar una conversación inexistente
        Then el sistema me indica que la conversación no existe
