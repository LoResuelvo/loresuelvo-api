@wip
Feature: Obtener detalle de conversación con el chatbot asistido por IA
    Como consumidor
    quiero consultar el detalle de una conversación existente con el chatbot asistido por IA
    para retomar el diagnóstico, revisar los mensajes y ver las recomendaciones ya obtenidas

    Background:
        Given que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "diego@example.com", nombre "Diego" y apellido "Sosa"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And existe un prestador registrado con correo "laura.electricista@example.com", nombre "Laura", apellido "Suárez" y rubro "Electricidad"
        And que el chatbot asistido por IA está disponible

    Rule: El detalle de una conversación se obtiene de forma transparente por el identificador de conversación

    Scenario: 11.4.1 - Consumidor obtiene el detalle de una conversación con el chatbot
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina" con los mensajes:
            """
            consumer: Hay agua acumulada dentro del mueble bajo mesada cada vez que uso la pileta.
            chatbot: Revisá si el agua sale desde la rosca del sifón o desde la manguera flexible.
            """
        When consulto el detalle de esa conversación
        Then el sistema muestra una conversación con el chatbot titulada "Pérdida de agua en la cocina"
        And el detalle de la conversación con el chatbot incluye mi mensaje:
            """
            Hay agua acumulada dentro del mueble bajo mesada cada vez que uso la pileta.
            """
        And el detalle de la conversación con el chatbot incluye la respuesta del chatbot asistido por IA:
            """
            Revisá si el agua sale desde la rosca del sifón o desde la manguera flexible.
            """

    Scenario: 11.4.2 - Consumidor vuelve a ver las recomendaciones obtenidas previamente
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot asistido por IA con diagnóstico concluido para el rubro "Plomería" y respuesta:
            """
            El problema parece ser una pérdida en el sifón o en una conexión flexible de la bacha. Te recomiendo contactar a un plomero.
            """
        When consulto el detalle de esa conversación
        Then el sistema muestra que el diagnóstico del chatbot está concluido
        And el sistema muestra el rubro recomendado "Plomería" en el detalle de la conversación con el chatbot
        And el sistema muestra al prestador recomendado "Juan Gómez" en el detalle de la conversación con el chatbot
        And el sistema muestra al prestador recomendado "Pedro Dib" en el detalle de la conversación con el chatbot
        And el sistema no muestra al prestador recomendado "Laura Suárez" en el detalle de la conversación con el chatbot

    Rule: Solo el consumidor dueño puede consultar una conversación con el chatbot

    Scenario: 11.4.3 - Rechazar consulta de otro consumidor
        Given que el consumidor "ana@example.com" tiene una conversación activa con el chatbot
        And que estoy autenticado como consumidor "diego@example.com"
        When intento consultar el detalle de esa conversación con el chatbot asistido por IA
        Then el sistema me indica que no puedo acceder a esa conversación con el chatbot asistido por IA

    Scenario: 11.4.4 - Rechazar consulta de prestador
        Given que el consumidor "ana@example.com" tiene una conversación activa con el chatbot
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento consultar el detalle de esa conversación con el chatbot asistido por IA
        Then el sistema me indica que no puedo acceder a esa conversación con el chatbot asistido por IA

    Scenario: 11.4.5 - Rechazar consulta sin sesión válida
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina" con los mensajes:
            """
            consumer: Hay agua acumulada dentro del mueble bajo mesada cada vez que uso la pileta.
            chatbot: Revisá si el agua sale desde la rosca del sifón o desde la manguera flexible.
            """
        And que no tengo una sesión válida
        When intento consultar el detalle de esa conversación con el chatbot asistido por IA
        Then el sistema deniega el acceso

    Rule: La conversación solicitada debe existir

    Scenario: 11.4.7 - Rechazar consulta de conversación inexistente
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento consultar una conversación con el chatbot asistido por IA inexistente
        Then el sistema me indica que la conversación no existe
