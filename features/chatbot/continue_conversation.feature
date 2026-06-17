Feature: Continuar conversación con el chatbot asistido por IA
    Como consumidor
    quiero enviar nuevos mensajes a una conversación existente con el chatbot asistido por IA
    para continuar recibiendo orientación preliminar sin perder el contexto del problema de mi hogar

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que el chatbot asistido por IA está disponible

    Rule: El consumidor puede continuar una conversación existente con el chatbot asistido por IA

    @wip
    Scenario: 11.2.1 - Continuar una conversación existente con el chatbot asistido por IA
        Given que estoy autenticado como consumidor "ana@example.com"
        And ya tengo una conversación activa con el chatbot sobre:
            """
            Tengo una pérdida de agua debajo de la pileta de la cocina.
            """
        And que el chatbot asistido por IA responderá:
            """
            Si el agua aparece solo al usar la bacha, revisá primero el sifón y las conexiones flexibles.
            """
        When envío un nuevo mensaje a esa conversación con el chatbot asistido por IA:
            """
            El agua aparece solamente cuando abro la canilla.
            """
        Then el sistema agrega mi nuevo mensaje a la misma conversación con el chatbot asistido por IA
        And el sistema registra la nueva respuesta del chatbot asistido por IA:
            """
            Si el agua aparece solo al usar la bacha, revisá primero el sifón y las conexiones flexibles.
            """
        And el sistema no crea una nueva conversación con el chatbot asistido por IA

    @wip
    Scenario: 11.2.2 - El chatbot recibe contexto de la conversación sin reenviar todo el historial
        Given que estoy autenticado como consumidor "ana@example.com"
        And ya tengo una conversación activa con el chatbot con muchos mensajes sobre una pérdida de agua en la cocina
        And que existe un resumen de contexto de esa conversación con el chatbot:
            """
            La consumidora reportó una pérdida debajo de la pileta de la cocina. El agua aparece al usar la bacha y todavía no identificó si proviene del sifón o de una conexión flexible.
            """
        When envío un nuevo mensaje a esa conversación con el chatbot asistido por IA:
            """
            Secamos la zona y vuelve a mojarse cerca de la rosca del sifón.
            """
        Then el sistema envía al chatbot el resumen de contexto de la conversación
        And el sistema envía al chatbot los mensajes recientes relevantes de la conversación

    @wip
    Scenario: 11.2.3 - Solo el consumidor dueño puede continuar una conversación con el chatbot
        Given que existe un consumidor registrado con correo "maria@example.com", nombre "María" y apellido "López"
        And que el consumidor "ana@example.com" tiene una conversación activa con el chatbot
        And que estoy autenticado como consumidor "maria@example.com"
        When intento enviar un nuevo mensaje a esa conversación con el chatbot asistido por IA:
            """
            Quiero continuar esta consulta.
            """
        Then el sistema me indica que no puedo acceder a esa conversación con el chatbot asistido por IA
        And el sistema no registra mi mensaje en esa conversación con el chatbot asistido por IA

    @wip
    Scenario: 11.2.4 - Una conversación con el chatbot no acepta dos mensajes simultáneos
        Given que estoy autenticado como consumidor "ana@example.com"
        And ya tengo una conversación activa con el chatbot
        And la conversación con el chatbot está procesando una respuesta anterior
        When intento enviar un nuevo mensaje a esa conversación con el chatbot asistido por IA:
            """
            También veo humedad en la pared cercana.
            """
        Then el sistema me indica que la conversación con el chatbot está procesando otro mensaje
        And el sistema no registra mi mensaje en esa conversación con el chatbot asistido por IA
