Feature: Obtener conversaciones
    Como usuario
    quiero ver mis conversaciones
    para estar al tanto los mensajes recibidos

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "diego@example.com", nombre "Diego" y apellido "Sosa"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Scenario: 01-OCS Usuario sin conversaciones obtiene un listado vacío
        Given que estoy autenticado como consumidor "diego@example.com"
        When consulto mis conversaciones
        Then el sistema muestra un listado de conversaciones vacío

    Rule: Un usuario autenticado puede listar las conversaciones en las que participa

    Scenario: 02-OCS Consumidor obtiene sus conversaciones
        Given que estoy autenticado como consumidor "ana@example.com"
        When consulto mis conversaciones
        Then el sistema muestra solamente la conversación con el prestador "Juan Gómez"
        And el último mensaje de la conversación es
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Scenario: 03-OCS Prestador obtiene sus conversaciones
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When consulto mis conversaciones
        Then el sistema muestra una conversación pendiente con el consumidor "Ana Pérez"
        And el último mensaje de la conversación es
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Rule: Solo usuarios autenticados pueden listar conversaciones

    Scenario: 06-OCS Rechazar listado sin sesión válida
        Given que no tengo una sesión válida
        When intento consultar mis conversaciones
        Then el sistema deniega el acceso
