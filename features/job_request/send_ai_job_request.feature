Feature: 51 Enviar solicitud de conversación de trabajo a Prestadores, con IA
    Como consumidor
    quiero contactar a uno o más prestadores recomendados a partir de la evaluación realizada por el chatbot
    para solicitarles ayuda sin tener que volver a describir el problema de mi hogar

    Background:
        Given que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And existe un prestador registrado con correo "laura.electricista@example.com", nombre "Laura", apellido "Suárez" y rubro "Electricidad"
        And que el chatbot asistido por IA está disponible

    Rule: Solo una evaluación vigente que requiere un profesional permite contactar prestadores

    @wip
    Scenario: 51.1 - Enviar una solicitud a un prestador recomendado usando la evaluación del chatbot
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "Plomería" con el título "Pérdida debajo de la pileta" y la descripción:
            """
            Hay una pérdida de agua debajo de la pileta cada vez que se abre la canilla. El agua parece provenir de la conexión del sifón y el problema continúa después de ajustarla manualmente.
            """
        When elijo contactar al prestador recomendado "Juan Gómez" desde esa conversación con el chatbot
        Then el sistema registra una solicitud de trabajo pendiente para el prestador "Juan Gómez"
        And la solicitud tiene el título "Pérdida debajo de la pileta"
        And la solicitud tiene la descripción obtenida de la evaluación vigente:
            """
            Hay una pérdida de agua debajo de la pileta cada vez que se abre la canilla. El agua parece provenir de la conexión del sifón y el problema continúa después de ajustarla manualmente.
            """
        And la solicitud queda vinculada con la evaluación vigente de la conversación con el chatbot
        And el sistema crea una conversación de trabajo pendiente entre "Ana Pérez" y "Juan Gómez"

    @wip
    Scenario: 51.2 - No permitir contactar prestadores cuando el problema puede ser resuelto por el consumidor
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot cuya evaluación vigente clasifica el problema en el rubro "Plomería" y determina que puede resolverse sin un profesional
        When intento contactar al prestador "Juan Gómez" desde esa conversación con el chatbot
        Then el sistema indica que la evaluación vigente no requiere contactar a un profesional
        And el sistema no registra una solicitud de trabajo
        And el sistema no crea una conversación de trabajo pendiente

    @wip
    Scenario: 51.3 - No permitir contactar prestadores mientras falta información para evaluar el problema
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot cuya evaluación vigente todavía requiere más información
        When intento contactar al prestador "Juan Gómez" desde esa conversación con el chatbot
        Then el sistema indica que todavía no existe información suficiente para contactar a un profesional
        And el sistema no registra una solicitud de trabajo
        And el sistema no crea una conversación de trabajo pendiente

    @wip
    Scenario: 51.4 - Una respuesta posterior fuera de alcance no invalida una evaluación profesional vigente
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "Plomería" con el título "Pérdida debajo de la pileta" y la descripción:
            """
            Hay una pérdida de agua persistente en la conexión del sifón debajo de la pileta.
            """
        And que la última respuesta del chatbot en esa conversación fue por una pregunta fuera del alcance de los problemas del hogar
        When elijo contactar al prestador recomendado "Juan Gómez" desde esa conversación con el chatbot
        Then el sistema registra una solicitud de trabajo pendiente para el prestador "Juan Gómez"
        And la solicitud conserva el título y la descripción de la evaluación profesional vigente

    Rule: El prestador contactado debe corresponder al rubro de la evaluación vigente

    @wip
    Scenario: 51.5 - Rechazar un prestador de un rubro diferente al evaluado
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "Plomería"
        When intento contactar al prestador "Laura Suárez" desde esa conversación con el chatbot
        Then el sistema indica que el prestador no corresponde al rubro requerido por la evaluación vigente
        And el sistema no registra una solicitud de trabajo para "Laura Suárez"
        And el sistema no crea una conversación de trabajo pendiente con "Laura Suárez"

    @wip
    Scenario: 51.6 - Enviar solicitudes a varios prestadores recomendados desde la misma evaluación
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "Plomería" con el título "Pérdida debajo de la pileta" y la descripción:
            """
            Hay una pérdida de agua persistente en la conexión del sifón debajo de la pileta.
            """
        When elijo contactar a los prestadores recomendados "Juan Gómez" y "Pedro Dib" desde esa conversación con el chatbot
        Then el sistema registra una solicitud de trabajo pendiente para el prestador "Juan Gómez"
        And el sistema registra una solicitud de trabajo pendiente para el prestador "Pedro Dib"
        And ambas solicitudes quedan vinculadas con la misma evaluación vigente
        And el sistema crea una conversación de trabajo pendiente con cada prestador contactado

    Rule: La solicitud utiliza una copia trazable de la evaluación seleccionada

    @wip
    Scenario: 51.7 - Utilizar la revisión vigente de la evaluación al contactar al prestador
        Given que estoy autenticado como consumidor "ana@example.com"
        And que mi conversación con el chatbot tenía una evaluación que permitía resolver el problema sin un profesional
        And que después de aportar nueva información la evaluación vigente requiere un profesional del rubro "Plomería" con el título "Pérdida persistente en el sifón" y la descripción:
            """
            La pérdida reapareció después de ajustar el sifón y ahora se produce incluso sin utilizar la pileta.
            """
        When elijo contactar al prestador recomendado "Juan Gómez" desde esa conversación con el chatbot
        Then el sistema registra la solicitud usando la revisión vigente de la evaluación
        And la solicitud tiene el título "Pérdida persistente en el sifón"
        And la solicitud tiene la descripción obtenida de la evaluación vigente:
            """
            La pérdida reapareció después de ajustar el sifón y ahora se produce incluso sin utilizar la pileta.
            """

    @wip
    Scenario: 51.8 - Conservar en la solicitud el contenido enviado aunque la evaluación evolucione posteriormente
        Given que estoy autenticado como consumidor "ana@example.com"
        And que envié al prestador "Juan Gómez" una solicitud desde una evaluación con el título "Pérdida en el sifón" y la descripción:
            """
            La pérdida aparece únicamente cuando se utiliza la pileta.
            """
        And que después de enviar la solicitud la conversación con el chatbot produjo una nueva revisión de la evaluación con el título "Pérdida persistente en la conexión" y la descripción:
            """
            La pérdida ahora aparece incluso sin utilizar la pileta.
            """
        When consulto la solicitud de trabajo enviada al prestador "Juan Gómez"
        Then la solicitud conserva el título "Pérdida en el sifón"
        And la solicitud conserva la descripción que fue enviada originalmente:
            """
            La pérdida aparece únicamente cuando se utiliza la pileta.
            """
        And la solicitud continúa vinculada con la revisión de la evaluación que la originó

    Rule: Solo el consumidor dueño de la conversación puede generar solicitudes desde su evaluación

    @wip
    Scenario: 51.9 - Rechazar el contacto desde la conversación con el chatbot de otro consumidor
        Given que el consumidor "ana@example.com" tiene una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "Plomería"
        And que estoy autenticado como consumidor "carla@example.com"
        When intento contactar al prestador "Juan Gómez" desde esa conversación con el chatbot
        Then el sistema indica que no puedo acceder a esa conversación con el chatbot
        And el sistema no registra una solicitud de trabajo
        And el sistema no crea una conversación de trabajo pendiente

    Rule: No puede existir más de una solicitud abierta entre el consumidor y el mismo prestador

    @wip
    Scenario: 51.10 - No duplicar una solicitud abierta para el mismo prestador
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "Plomería"
        And que ya existe una solicitud de trabajo abierta entre "Ana Pérez" y "Juan Gómez"
        When intento contactar al prestador "Juan Gómez" desde esa conversación con el chatbot
        Then el sistema indica que ya existe una solicitud de trabajo abierta con ese prestador
        And el sistema no registra otra solicitud de trabajo para "Juan Gómez"
        And el sistema no crea otra conversación de trabajo pendiente con "Juan Gómez"
