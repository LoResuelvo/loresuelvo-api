Feature: 50.2 Enviar videos por el chat
    Como participante de un chat de trabajo
    quiero enviar videos en mis mensajes
    para mostrar el problema o compartir indicaciones con la otra parte

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    Rule: Los participantes pueden enviar un único video solo o acompañado de texto

    @wip
    Scenario: 50.2.1-EVC Consumidor envía únicamente un video en un chat activo
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el video sin audio "perdida-canilla.mp4" de 18 segundos
        When envío únicamente el video "perdida-canilla.mp4" en el chat con el prestador "Juan Gómez"
        Then el sistema registra el mensaje con el video "perdida-canilla.mp4" en el chat
        And el mensaje fue enviado por el consumidor "Ana Pérez"
        And el video queda asociado al mensaje enviado

    @wip
    Scenario: 50.2.2-EVC Prestador envía un video acompañado de texto en un chat activo
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como prestador "juan.plomero@example.com"
        And que cargué y confirmé el video "reparacion-propuesta.mp4" de 24 segundos
        When envío el video "reparacion-propuesta.mp4" en el chat con la consumidora "Ana Pérez" acompañado del texto:
            """
            Te muestro cómo quedaría realizada la reparación.
            """
        Then el sistema registra el mensaje con el video "reparacion-propuesta.mp4" y el texto enviado
        And el mensaje fue enviado por el prestador "Juan Gómez"
        And el video queda asociado al mensaje enviado

    Rule: Los mensajes con video respetan el estado del chat de trabajo

    @wip
    Scenario: 50.2.3-EVC Consumidor envía un video acompañado de texto en un chat pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el video "detalle-perdida.mp4" de 20 segundos
        When envío el video "detalle-perdida.mp4" en la conversación pendiente con el prestador "Juan Gómez" acompañado del texto:
            """
            La pérdida empieza cuando abro esta canilla.
            """
        Then el sistema registra el mensaje con el video "detalle-perdida.mp4" y el texto enviado en la conversación pendiente

    @wip
    Scenario: 50.2.4-EVC Rechazar video del prestador en un chat pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        And que cargué y confirmé el video "respuesta-prestador.mp4" de 12 segundos
        When intento enviar únicamente el video "respuesta-prestador.mp4" en la conversación pendiente con la consumidora "Ana Pérez"
        Then el sistema rechaza el mensaje porque el prestador debe aceptar la solicitud de trabajo antes de responder
        And el sistema no asocia el video a ningún mensaje

    @wip
    Scenario: 50.2.5-EVC Contabilizar el video para el límite de mensajes del consumidor en un chat pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que el consumidor "ana@example.com" ya alcanzó el límite de mensajes permitido en esa conversación pendiente
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el video "mensaje-adicional.mp4" de 8 segundos
        When intento enviar únicamente el video "mensaje-adicional.mp4" en la conversación pendiente con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque se alcanzó el límite de mensajes de la conversación pendiente
        And el sistema no asocia el video a ningún mensaje

    Rule: El video solo puede combinarse con texto

    @wip
    Scenario: 50.2.6-EVC Rechazar un mensaje que combina video con texto e imágenes
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el video "perdida-canilla.mp4" de 18 segundos
        And que cargué y confirmé la imagen "detalle-conexion.jpg"
        When intento enviar el video "perdida-canilla.mp4" junto con la imagen "detalle-conexion.jpg" y el texto:
            """
            La pérdida comienza en esta conexión.
            """
        Then el sistema rechaza el mensaje porque el video no puede enviarse con imágenes
        And el sistema no asocia el video ni la imagen a ningún mensaje

    @wip
    Scenario: 50.2.7-EVC Rechazar un mensaje que combina video con audio
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el video "perdida-canilla.mp4" de 18 segundos
        And que cargué y confirmé el audio "ruido-bomba.webm" de 18 segundos
        When intento enviar el video "perdida-canilla.mp4" junto con el audio "ruido-bomba.webm"
        Then el sistema rechaza el mensaje porque el audio debe enviarse sin texto, imágenes ni video
        And el sistema no asocia el video ni el audio a ningún mensaje

    Rule: La contraparte puede recibir y consultar los videos del chat

    @wip
    Scenario: 50.2.8-EVC La contraparte consulta un mensaje con video
        Given que el consumidor "ana@example.com" envió el video "perdida-canilla.mp4" en el chat con el prestador "juan.plomero@example.com"
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When consulto el chat activo con el consumidor "ana@example.com"
        Then el detalle incluye el mensaje con el video "perdida-canilla.mp4"
        And el detalle muestra la duración, dimensiones, formato MP4 y codecs del video
        And el sistema permite al prestador acceder al video adjunto

    @wip
    Scenario: 50.2.9-EVC La contraparte recibe en tiempo real un mensaje con video
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el video "perdida-canilla.mp4" de 18 segundos
        When envío únicamente el video "perdida-canilla.mp4" en el chat con el prestador "Juan Gómez"
        Then el prestador "juan.plomero@example.com" recibe en tiempo real el mensaje con el video "perdida-canilla.mp4"
        And el evento recibido incluye los metadatos y el acceso al video

    @wip
    Scenario: 50.2.10-EVC Mostrar un último mensaje con video en el listado de conversaciones
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo un chat activo con el prestador "juan.plomero@example.com" cuyo último mensaje contiene el video "perdida-canilla.mp4" de 18 segundos
        When consulto mis conversaciones
        Then el último mensaje de esa conversación se identifica como un mensaje con video
        And el último mensaje informa una duración de 18 segundos

    Rule: Solo pueden enviarse videos confirmados y pertenecientes al remitente

    @wip
    Scenario: 50.2.11-EVC Rechazar un video que todavía no fue confirmado
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué pero no confirmé el video "carga-incompleta.mp4"
        When intento enviar únicamente el video "carga-incompleta.mp4" en el chat con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque el video no está disponible
        And el sistema no asocia el video a ningún mensaje

    @wip
    Scenario: 50.2.12-EVC Rechazar un video cargado por otro usuario
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que la consumidora "carla@example.com" cargó y confirmó el video "video-ajeno.mp4"
        And que estoy autenticado como consumidor "ana@example.com"
        When intento enviar únicamente el video "video-ajeno.mp4" en el chat con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque el video no está disponible
        And el sistema no asocia el video a ningún mensaje

    @wip
    Scenario: 50.2.13-EVC Rechazar un archivo cargado para otra finalidad
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el archivo "presentacion.mp4" para otra finalidad
        When intento enviar únicamente el archivo "presentacion.mp4" como video en el chat con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque el video no está disponible
        And el sistema no asocia el archivo a ningún mensaje

    Rule: Los videos son privados para los participantes del chat

    @wip
    Scenario Outline: 50.2.14-EVC Rechazar el acceso de un <rol> ajeno al video
        Given que el consumidor "ana@example.com" envió el video "perdida-canilla.mp4" en el chat con el prestador "juan.plomero@example.com"
        And que estoy autenticado como <rol> "<correo>"
        When intento acceder al video "perdida-canilla.mp4" adjunto al mensaje
        Then el sistema me indica que no puedo acceder a ese video

        Examples:
            | rol        | correo                    |
            | consumidor | carla@example.com         |
            | prestador  | pedro.plomero@example.com |

    Rule: Los videos deben usar MP4 con H.264 y AAC opcional, no superar 50 MiB, 120 segundos ni resolución Full HD

    @wip
    Scenario Outline: 50.2.15-EVC Rechazar un formato o codec de video no soportado
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento cargar el video "<archivo>" con formato <formato>, codec de video <video_codec> y codec de audio <audio_codec> para un mensaje del chat
        Then el sistema rechaza la carga porque el video no usa MP4 con H.264 y AAC opcional

        Examples:
            | archivo            | formato | video_codec | audio_codec |
            | grabacion.webm     | WebM    | H.264       | AAC         |
            | grabacion-hevc.mp4 | MP4     | HEVC        | AAC         |
            | grabacion-opus.mp4 | MP4     | H.264       | Opus        |

    @wip
    Scenario: 50.2.16-EVC Rechazar un archivo cuyo contenido no es un video válido
        Given que estoy autenticado como consumidor "ana@example.com"
        And que solicité cargar "contenido-invalido.mp4" como video MP4 con H.264
        When intento confirmar un archivo cuyo contenido no corresponde a un video MP4 válido
        Then el sistema rechaza la confirmación porque el contenido del video no es válido

    @wip
    Scenario: 50.2.17-EVC Rechazar un video que supera el tamaño máximo permitido
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento cargar un video MP4 con H.264 de 51 MiB para un mensaje del chat
        Then el sistema rechaza la carga porque el video supera el máximo de 50 MiB

    @wip
    Scenario: 50.2.18-EVC Rechazar un video que supera la duración máxima permitida
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué el video MP4 con H.264 "video-extenso.mp4" de 121 segundos
        When intento confirmar el video para un mensaje del chat
        Then el sistema rechaza la confirmación porque el video supera el máximo de 120 segundos

    @wip
    Scenario: 50.2.19-EVC Rechazar un video que supera la resolución máxima permitida
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué el video MP4 con H.264 "video-4k.mp4" con resolución 3840 por 2160
        When intento confirmar el video para un mensaje del chat
        Then el sistema rechaza la confirmación porque el video supera la resolución Full HD

    @wip
    Scenario: 50.2.20-EVC Aceptar un video que cumple exactamente los límites permitidos
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el video MP4 con H.264 y AAC "video-limite.mp4" de 50 MiB, 120 segundos y resolución 1080 por 1920
        When envío únicamente el video "video-limite.mp4" en el chat con el prestador "Juan Gómez"
        Then el sistema registra el mensaje con el video "video-limite.mp4" en el chat
