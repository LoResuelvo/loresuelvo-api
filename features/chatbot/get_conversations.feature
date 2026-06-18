Feature: Obtener conversaciones con el chatbot asistido por IA
    Como consumidor
    quiero ver mis conversaciones con el chatbot asistido por IA
    para retomar consultas previas sobre problemas de mi hogar

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "diego@example.com", nombre "Diego" y apellido "Sosa"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que el chatbot asistido por IA está disponible

    Rule: Un consumidor autenticado puede listar sus conversaciones con el chatbot asistido por IA

    Scenario: 11.3.1 - Consumidor sin conversaciones con el chatbot obtiene un listado vacío
        Given que estoy autenticado como consumidor "diego@example.com"
        When consulto mis conversaciones con el chatbot asistido por IA
        Then el sistema muestra un listado de conversaciones con el chatbot asistido por IA vacío

    Scenario: 11.3.2 - Consumidor obtiene sus conversaciones con el chatbot asistido por IA
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina" con el último mensaje:
            """
            Revisá si el agua sale desde la rosca del sifón o desde la manguera flexible.
            """
        When consulto mis conversaciones con el chatbot asistido por IA
        Then el sistema muestra la conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina"
        And el último mensaje de la conversación con el chatbot asistido por IA es:
            """
            Revisá si el agua sale desde la rosca del sifón o desde la manguera flexible.
            """

    Scenario: 11.3.3 - Consumidor ve solamente sus propias conversaciones con el chatbot
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina" con el último mensaje:
            """
            Cerrá la llave de paso y revisá el sifón.
            """
        And que el consumidor "diego@example.com" tiene una conversación con el chatbot asistido por IA titulada "Problema eléctrico en cocina"
        When consulto mis conversaciones con el chatbot asistido por IA
        Then el sistema muestra la conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina"
        And el sistema no muestra la conversación con el chatbot asistido por IA titulada "Problema eléctrico en cocina"

    Scenario: 11.3.4 - Las conversaciones de trabajo no aparecen en el listado del chatbot
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina" con el último mensaje:
            """
            Te recomiendo contactar a un plomero si la pérdida continúa.
            """
        And que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        When consulto mis conversaciones con el chatbot asistido por IA
        Then el sistema muestra la conversación con el chatbot asistido por IA titulada "Pérdida de agua en la cocina"
        And el sistema no muestra la conversación con el prestador "Juan Gómez" en el listado de conversaciones con el chatbot asistido por IA

    Rule: Solo consumidores autenticados pueden listar conversaciones con el chatbot asistido por IA

    Scenario: 11.3.5 - Rechazar listado de conversaciones con el chatbot sin sesión válida
        Given que no tengo una sesión válida
        When intento consultar mis conversaciones con el chatbot asistido por IA
        Then el sistema deniega el acceso

    Scenario: 11.3.6 - Rechazar listado de conversaciones con el chatbot para prestadores
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When intento consultar mis conversaciones con el chatbot asistido por IA
        Then el sistema me indica que solo un consumidor puede consultar conversaciones con el chatbot asistido por IA
