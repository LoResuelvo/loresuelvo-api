@wip
Feature: Chatbot asistido por IA
    Como consumidor
    quiero enviar mensajes a un chatbot asistido por IA
    para recibir orientación preliminar sobre el problema de mi hogar

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que el chatbot asistido por IA está disponible

    Scenario: 11.1.1 - Crear conversación con el chatbot asistido por IA
        Given que estoy autenticado como consumidor "ana@example.com"
        When envío un mensaje al chatbot asistido por IA:
            """
            Tengo una pérdida de agua debajo de la pileta de la cocina y no sé qué revisar primero.
            """
        Then el sistema crea una conversación con el chatbot asistido por IA para el consumidor "ana@example.com"
        And la conversación contiene mi mensaje:
            """
            Tengo una pérdida de agua debajo de la pileta de la cocina y no sé qué revisar primero.
            """

    Scenario: 11.1.2 - Recibir respuesta a mi consulta
        Given que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA responderá:
            """
            Podría tratarse de una pérdida en el sifón o en una conexión flexible. Cerrá la llave de paso y revisá si el goteo viene de una unión.
            """
        When envío un mensaje al chatbot asistido por IA:
            """
            La pileta de la cocina pierde agua por abajo cuando abro la canilla.
            """
        Then el sistema muestra la respuesta del chatbot asistido por IA:
            """
            Podría tratarse de una pérdida en el sifón o en una conexión flexible. Cerrá la llave de paso y revisá si el goteo viene de una unión.
            """

    Scenario: 11.1.3 - Los mensajes de la conversación se registran
        Given que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA responderá:
            """
            Revisá si el agua sale desde la rosca del sifón o desde la manguera flexible. Si continúa, contactá a un plomero.
            """
        When envío un mensaje al chatbot asistido por IA:
            """
            Sale agua cuando uso la bacha y el mueble se moja por dentro.
            """
        Then el sistema registra mi mensaje en la conversación con el chatbot asistido por IA:
            """
            Sale agua cuando uso la bacha y el mueble se moja por dentro.
            """
        And el sistema registra la respuesta del chatbot asistido por IA:
            """
            Revisá si el agua sale desde la rosca del sifón o desde la manguera flexible. Si continúa, contactá a un plomero.
            """

    Scenario: 11.1.4 - Solo un consumidor puede enviar mensajes al chatbot asistido por IA
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar un mensaje al chatbot asistido por IA:
            """
            Quiero probar el asistente para responder consultas de clientes.
            """
        Then el sistema me indica que solo un consumidor puede enviar mensajes al chatbot asistido por IA
        And el sistema no crea una conversación con el chatbot asistido por IA

    Scenario: 11.1.5 - Generación de respuesta pre diagnóstico
        Given que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA responderá con un pre diagnóstico:
            """
            Pre diagnóstico: posible pérdida en el sifón o en una junta de la pileta. Recomendación: cerrar la llave de paso, secar la zona y contactar a un plomero si el goteo continúa.
            """
        When envío un mensaje al chatbot asistido por IA:
            """
            Hay humedad dentro del mueble bajo mesada y aparece agua después de lavar los platos.
            """
        Then el sistema muestra un pre diagnóstico del problema del hogar:
            """
            Pre diagnóstico: posible pérdida en el sifón o en una junta de la pileta. Recomendación: cerrar la llave de paso, secar la zona y contactar a un plomero si el goteo continúa.
            """

    Rule: El chatbot solamente puede responder preguntas relacionadas con el problema del hogar del consumidor

    Scenario: 11.1.6 - El chatbot rechaza preguntas fuera del ámbito del problema del hogar del consumidor
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento enviar una pregunta fuera del ámbito del problema del hogar al chatbot asistido por IA:
            """
            ¿Qué equipo ganó el último partido de fútbol?
            """
        Then el sistema me indica que el chatbot solo responde preguntas relacionadas con problemas del hogar

    Scenario: 11.1.7 - Puedo tener mas de una conversación con el chatbot asistido por IA
        Given que estoy autenticado como consumidor "ana@example.com"
        And ya tengo una conversación activa con el chatbot
        When inicio una nueva conversacion enviando un mensaje al chatbot asistido por IA:
            """
            También tengo un problema con la luz en la cocina. ¿Qué puedo revisar?
            """
        Then el sistema crea una nueva conversación con el chatbot asistido por IA para el nuevo mensaje
        And tengo un total de dos conversaciones activas con el chatbot asistido por IA

    Scenario: 11.1.8 - Titulo de la conversacion
        Given que estoy autenticado como consumidor "ana@example.com"
        When envío un mensaje al chatbot asistido por IA:
            """
            Tengo una pérdida de agua debajo de la pileta de la cocina y no sé qué revisar primero.
            """
        Then el sistema crea una conversación con el chatbot asistido por IA con el título "Pérdida de agua en la cocina"
