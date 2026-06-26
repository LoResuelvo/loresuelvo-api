Feature: 12.1 Adjuntar imágenes para pre-diagnóstico
    Como consumidor
    quiero adjuntar imágenes a mis mensajes con el chatbot asistido por IA
    para que el pre-diagnóstico considere evidencia visual del problema de mi hogar

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que el chatbot asistido por IA está disponible

    Rule: El consumidor puede adjuntar imágenes al iniciar o continuar el pre-diagnóstico

    Scenario: 12.1.1 - Crear conversación con el chatbot adjuntando una imagen
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé la imagen "perdida-bajo-mesada.jpg"
        And que el chatbot asistido por IA responderá:
            """
            La imagen muestra una posible pérdida en una conexión bajo mesada. Cerrá la llave de paso y revisá si el goteo viene del sifón o de la manguera flexible.
            """
        When envío un mensaje al chatbot asistido por IA con la imagen cargada "perdida-bajo-mesada.jpg":
            """
            ¿Podés orientarme con esta pérdida debajo de la pileta?
            """
        Then el sistema crea una conversación con el chatbot asistido por IA para el consumidor "ana@example.com"
        And la conversación contiene mi mensaje con la imagen "perdida-bajo-mesada.jpg"
        And el chatbot recibe la imagen "perdida-bajo-mesada.jpg" para elaborar el pre-diagnóstico
        And el sistema muestra la respuesta del chatbot asistido por IA:
            """
            La imagen muestra una posible pérdida en una conexión bajo mesada. Cerrá la llave de paso y revisá si el goteo viene del sifón o de la manguera flexible.
            """

    Scenario: 12.1.2 - Continuar una conversación con el chatbot adjuntando una imagen
        Given que estoy autenticado como consumidor "ana@example.com"
        And ya tengo una conversación activa con el chatbot sobre:
            """
            Tengo una pérdida de agua debajo de la pileta de la cocina.
            """
        And que cargué y confirmé la imagen "detalle-sifon.jpg"
        And que el chatbot asistido por IA responderá:
            """
            Por la imagen, la humedad parece concentrarse cerca del sifón. Secá la zona, ajustá la rosca con cuidado y contactá a un plomero si vuelve a gotear.
            """
        When envío un nuevo mensaje a esa conversación con el chatbot asistido por IA con la imagen cargada "detalle-sifon.jpg":
            """
            Saqué una foto de donde vuelve a mojarse.
            """
        Then el sistema agrega mi nuevo mensaje a la misma conversación con el chatbot asistido por IA
        And el mensaje queda asociado a la imagen "detalle-sifon.jpg"
        And el chatbot recibe la imagen "detalle-sifon.jpg" para elaborar el pre-diagnóstico
        And el sistema registra la nueva respuesta del chatbot asistido por IA:
            """
            Por la imagen, la humedad parece concentrarse cerca del sifón. Secá la zona, ajustá la rosca con cuidado y contactá a un plomero si vuelve a gotear.
            """

    Scenario: 12.1.3 - Enviar al chatbot un mensaje compuesto solamente por imágenes
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé la imagen "humedad-pared.webp"
        And que el chatbot asistido por IA responderá con un pre diagnóstico:
            """
            Pre diagnóstico: la imagen muestra humedad compatible con filtración o pérdida cercana. Conviene revisar si hay caños, grifería o desagües detrás de esa pared.
            """
        When envío un mensaje sin texto al chatbot asistido por IA con la imagen cargada "humedad-pared.webp"
        Then el sistema crea una conversación con el chatbot asistido por IA para el consumidor "ana@example.com"
        And la conversación contiene mi mensaje con la imagen "humedad-pared.webp"
        And el sistema muestra un pre diagnóstico del problema del hogar:
            """
            Pre diagnóstico: la imagen muestra humedad compatible con filtración o pérdida cercana. Conviene revisar si hay caños, grifería o desagües detrás de esa pared.
            """

    Scenario: 12.1.4 - Adjuntar más de una imagen en un mensaje al chatbot
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé las imágenes: "vista-general-cocina.jpg", "detalle-conexion.jpg"
        When envío un mensaje al chatbot asistido por IA con las imágenes cargadas:
            """
            Te envío una vista general y un detalle de la conexión que pierde agua.
            """
        Then la conversación contiene mi mensaje con las dos imágenes
        And el chatbot recibe las imágenes "vista-general-cocina.jpg" y "detalle-conexion.jpg" para elaborar el pre-diagnóstico

    Rule: Las imágenes del pre-diagnóstico quedan registradas y son privadas

    Scenario: 12.1.5 - Consultar una conversación con el chatbot que tiene imágenes adjuntas
        Given que el consumidor "ana@example.com" envió un mensaje al chatbot con la imagen "perdida-bajo-mesada.jpg"
        And que estoy autenticado como consumidor "ana@example.com"
        When consulto el detalle de esa conversación
        Then el detalle de la conversación con el chatbot incluye mi mensaje con la imagen "perdida-bajo-mesada.jpg"
        And el sistema permite al consumidor acceder a la imagen adjunta

    Scenario: 12.1.6 - Rechazar el acceso de otro consumidor a una imagen adjunta al chatbot
        Given que el consumidor "ana@example.com" envió un mensaje al chatbot con la imagen "perdida-bajo-mesada.jpg"
        And que estoy autenticado como consumidor "carla@example.com"
        When intento acceder a la imagen "perdida-bajo-mesada.jpg" adjunta al mensaje del chatbot
        Then el sistema me indica que no puedo acceder a esa imagen

    Rule: Solo pueden adjuntarse imágenes disponibles y pertenecientes al consumidor

    Scenario: 12.1.7 - Rechazar una imagen del chatbot que todavía no fue confirmada
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué pero no confirmé la imagen "carga-incompleta.jpg"
        When intento enviar un mensaje al chatbot asistido por IA adjuntando la imagen "carga-incompleta.jpg":
            """
            ¿Podés revisar esta imagen?
            """
        Then el sistema rechaza el mensaje porque la imagen no está disponible
        And el sistema no crea una conversación con el chatbot asistido por IA

    Scenario: 12.1.8 - Rechazar una imagen del chatbot cargada por otro usuario
        Given que la consumidora "carla@example.com" cargó y confirmó la imagen "imagen-ajena.jpg"
        And que estoy autenticado como consumidor "ana@example.com"
        When intento enviar un mensaje al chatbot asistido por IA adjuntando la imagen "imagen-ajena.jpg":
            """
            ¿Podés revisar esta imagen?
            """
        Then el sistema rechaza el mensaje porque la imagen no está disponible
        And el sistema no crea una conversación con el chatbot asistido por IA

    Scenario: 12.1.9 - Rechazar un archivo cargado para otra finalidad
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé la imagen "foto-de-perfil.jpg" como foto de perfil
        When intento enviar un mensaje al chatbot asistido por IA adjuntando la imagen "foto-de-perfil.jpg":
            """
            ¿Podés revisar esta imagen?
            """
        Then el sistema rechaza el mensaje porque la imagen no está disponible
        And el sistema no crea una conversación con el chatbot asistido por IA

    Rule: La cantidad máxima de imágenes que puede adjuntar un consumidor en un mensaje al chatbot asistido por IA es 5

    Scenario: 12.1.10 - Rechazar más imágenes que el máximo permitido
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé las imágenes: "img-1.jpg", "img-2.jpg", "img-3.jpg", "img-4.jpg", "img-5.jpg", "img-6.jpg"
        When intento enviar un mensaje al chatbot asistido por IA adjuntando esas imágenes:
            """
            Te envío varias fotos del problema.
            """
        Then el sistema rechaza el mensaje porque la imagen no está disponible
        And el sistema no crea una conversación con el chatbot asistido por IA
