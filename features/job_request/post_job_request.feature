Feature: Enviar solicitud de trabajo
    Como consumidor
    quiero enviar una solicitud de trabajo a un prestador
    para solicitar un servicio específico

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "diego@example.com", nombre "Diego" y apellido "López"
        And existe un prestador registrado con correo "ana.perez@example.com", nombre "Ana", apellido "Pérez" y rubro "Plomería"
        And que estoy autenticado como consumidor "diego@example.com"

    Scenario: 39.1 - EST Enviar solicitud de trabajo con datos válidos
        When envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y la descripción:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        Then el sistema registra la solicitud de trabajo

    Scenario: 39.2 - EST Enviar solicitud de trabajo sin título
        When envío una solicitud de trabajo al prestador "Ana Pérez" sin título y con la descripción:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        Then el sistema muestra un mensaje de error indicando que el título es obligatorio
    
    Scenario: 39.3 - EST Enviar solicitud de trabajo sin descripción
        When envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y sin descripción
        Then el sistema registra la solicitud de trabajo con una descripción vacía

    Scenario: 39.4 - EST Enviar solicitud de trabajo a un prestador inexistente
        When envío una solicitud de trabajo al prestador "Carlos López" con el título "Reparación de fuga en la cocina" y la descripción:
            """
            Hola Carlos, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        Then el sistema muestra un mensaje de error indicando que el prestador no existe

    Scenario: 39.5 - EST No permitir enviar otra solicitud si ya existe una solicitud abierta
        Given envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y la descripción:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        When envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Nuevo arreglo en la cocina" y la descripción:
            """
            También necesito revisar otra pérdida de agua.
            """
        Then el sistema muestra un mensaje de error indicando que ya existe una solicitud de trabajo abierta

    Scenario: 39.6 - EST Enviar hasta 5 mensajes por el chat pendiente
        Given envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y la descripción:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        When envio al chat pendiente con el prestador "Ana Pérez" los mensajes:
            """
            Mensaje 1
            Mensaje 2
            Mensaje 3
            Mensaje 4
            Mensaje 5
            Mensaje 6
            """
        Then el sistema muestra un mensaje de error indicando que se ha alcanzado el límite de mensajes permitidos en el chat pendiente
        And el sistema no registra el sexto mensaje en la conversación pendiente

    Scenario: 39.7 - EST Escribir en chat sin aceptar solicitud de trabajo vinculada
        Given envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y la descripción:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        When el prestador "Ana Pérez" intenta enviar un mensaje en el chat pendiente con el consumidor "Diego" sin aceptar la solicitud de trabajo vinculada
        Then el sistema muestra un mensaje de error indicando que no se puede enviar mensajes en el chat pendiente sin aceptar la solicitud de trabajo vinculada
    Scenario: 39.8 - EST Enviar solicitud de trabajo con una imagen adjunta
        Given que cargué y confirmé la imagen de solicitud de trabajo "perdida-bajo-mesada.jpg"
        When envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y la imagen cargada "perdida-bajo-mesada.jpg":
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. Te adjunto una imagen del problema.
            """
        Then el sistema registra la solicitud de trabajo con la imagen "perdida-bajo-mesada.jpg"
    Scenario: 39.9 - EST Enviar solicitud de trabajo con múltiples imágenes adjuntas
        Given que cargué y confirmé las imágenes de solicitud de trabajo: "perdida-bajo-mesada.jpg", "detalle-sifon.webp", "humedad-pared.png"
        When envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y las imágenes cargadas:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. Te adjunto varias imágenes del problema.
            """
        Then el sistema registra la solicitud de trabajo con las imágenes adjuntas
    Scenario: 39.10 - EST Rechazar solicitud de trabajo con más de 3 imágenes
        Given que cargué y confirmé las imágenes de solicitud de trabajo: "perdida-bajo-mesada.jpg", "detalle-sifon.webp", "humedad-pared.png", "caño-roto.jpg"
        When intento enviar una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y las imágenes cargadas:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. Te adjunto imágenes del problema.
            """
        Then el sistema rechaza la solicitud de trabajo porque supera el límite de imágenes
        And el sistema no asocia las imágenes a ninguna solicitud de trabajo
