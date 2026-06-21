Feature: 50 Adjuntar imágenes en el chat con un prestador
    Como participante de un chat de trabajo
    quiero adjuntar imágenes a mis mensajes
    para mostrar el problema o compartir información visual con la otra parte

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Rule: Los participantes pueden adjuntar imágenes a sus mensajes

    Scenario: 50.1-AICP Consumidor envía un mensaje con una imagen
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé la imagen "perdida-bajo-mesada.jpg"
        When envío un mensaje en el chat con el prestador "Juan Gómez" con la imagen cargada "perdida-bajo-mesada.jpg":
            """
            La pérdida se ve en esta conexión debajo de la pileta.
            """
        Then el sistema registra el mensaje con la imagen "perdida-bajo-mesada.jpg"
        And la imagen queda asociada al mensaje enviado

    Scenario: 50.2-AICP Prestador envía un mensaje con una imagen
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        And que cargué y confirmé la imagen "repuesto-recomendado.png"
        When envío un mensaje en el chat con la consumidora "Ana Pérez" con la imagen cargada "repuesto-recomendado.png":
            """
            Este es el repuesto que probablemente necesitemos.
            """
        Then el sistema registra el mensaje con la imagen "repuesto-recomendado.png"
        And la imagen queda asociada al mensaje enviado

    Scenario: 50.3-AICP Enviar un mensaje compuesto solamente por imágenes
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé la imagen "detalle-sifon.webp"
        When envío un mensaje sin texto en el chat con el prestador "Juan Gómez" con la imagen cargada "detalle-sifon.webp"
        Then el sistema registra el mensaje con la imagen "detalle-sifon.webp"

    Scenario: 50.4-AICP Enviar más de una imagen en el mismo mensaje
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé las imágenes: "vista-general-cocina.jpg", "detalle-conexion.jpg"
        When envío un mensaje en el chat con el prestador "Juan Gómez" con las imágenes cargadas:
            """
            Te envío una vista general y un detalle de la conexión.
            """
        Then el sistema registra el mensaje con las dos imágenes

    Rule: La contraparte recibe las imágenes adjuntas al mensaje

    Scenario: 50.5-AICP La contraparte consulta un mensaje con imágenes
        Given que el consumidor "ana@example.com" envió un mensaje con la imagen "perdida-bajo-mesada.jpg" en el chat
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When consulto el chat activo con el consumidor "ana@example.com"
        Then el detalle del mensaje incluye la imagen "perdida-bajo-mesada.jpg"
        And el sistema permite al prestador acceder a la imagen adjunta

    Scenario: 50.6-AICP La contraparte recibe en tiempo real un mensaje con imágenes
        Given que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé la imagen "perdida-bajo-mesada.jpg"
        When envío un mensaje en el chat con el prestador "Juan Gómez" con la imagen cargada "perdida-bajo-mesada.jpg":
            """
            Te envío una foto del lugar exacto de la pérdida.
            """
        Then el prestador "juan.plomero@example.com" recibe en tiempo real el mensaje con la imagen "perdida-bajo-mesada.jpg"

    Rule: Solo pueden adjuntarse imágenes confirmadas y pertenecientes al remitente

    Scenario: 50.7-AICP Rechazar una imagen que todavía no fue confirmada
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué pero no confirmé la imagen "carga-incompleta.jpg"
        When intento enviar un mensaje en el chat con el prestador "Juan Gómez" adjuntando la imagen "carga-incompleta.jpg"
        Then el sistema rechaza el mensaje porque la imagen no está disponible
        And el sistema no asocia la imagen a ningún mensaje

    Scenario: 50.8-AICP Rechazar una imagen cargada por otro usuario
        Given que la consumidora "carla@example.com" cargó y confirmó la imagen "imagen-ajena.jpg"
        And que estoy autenticado como consumidor "ana@example.com"
        When intento enviar un mensaje en el chat con el prestador "Juan Gómez" adjuntando la imagen "imagen-ajena.jpg"
        Then el sistema rechaza el mensaje porque la imagen no está disponible
        And el sistema no asocia la imagen a ningún mensaje

    Scenario: 50.9-AICP Rechazar un archivo cargado para otra finalidad
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        And que cargué y confirmé la imagen "foto-de-perfil.jpg" como foto de perfil
        When intento enviar un mensaje en el chat con la consumidora "Ana Pérez" adjuntando la imagen "foto-de-perfil.jpg"
        Then el sistema rechaza el mensaje porque la imagen no está disponible
        And el sistema no asocia la imagen a ningún mensaje

    Rule: Las imágenes adjuntas son privadas para los participantes del chat

    Scenario: 50.10-AICP Rechazar el acceso de un usuario ajeno a una imagen del chat
        Given que el consumidor "ana@example.com" envió un mensaje con la imagen "perdida-bajo-mesada.jpg" en el chat
        And que estoy autenticado como consumidor "carla@example.com"
        When intento acceder a la imagen "perdida-bajo-mesada.jpg" adjunta al mensaje
        Then el sistema me indica que no puedo acceder a esa imagen
