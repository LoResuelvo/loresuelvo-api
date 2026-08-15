Feature: 50.1 Enviar audios por el chat
    Como participante de un chat de trabajo
    quiero enviar mensajes de audio
    para comunicar detalles del problema o coordinar el servicio de manera más sencilla

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    Rule: Los participantes pueden enviar un único audio en el chat de trabajo

    Scenario: 50.1.1-EAC Consumidor envía un audio en un chat activo
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio "ruido-bomba.webm" de 18 segundos
        When envío únicamente el audio "ruido-bomba.webm" en el chat con el prestador "Juan Gómez"
        Then el sistema registra el mensaje de audio "ruido-bomba.webm" en el chat
        And el mensaje fue enviado por el consumidor "Ana Pérez"
        And el audio queda asociado al mensaje enviado

    Scenario: 50.1.2-EAC Prestador envía un audio en un chat activo
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como prestador "juan.plomero@example.com"
        And que cargué y confirmé el audio "indicaciones-visita.webm" de 24 segundos
        When envío únicamente el audio "indicaciones-visita.webm" en el chat con la consumidora "Ana Pérez"
        Then el sistema registra el mensaje de audio "indicaciones-visita.webm" en el chat
        And el mensaje fue enviado por el prestador "Juan Gómez"
        And el audio queda asociado al mensaje enviado

    Rule: Los mensajes de audio respetan el estado del chat de trabajo

    Scenario: 50.1.3-EAC Consumidor envía un audio en un chat pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio "detalle-perdida.webm" de 20 segundos
        When envío únicamente el audio "detalle-perdida.webm" en la conversación pendiente con el prestador "Juan Gómez"
        Then el sistema registra el mensaje de audio "detalle-perdida.webm" en la conversación pendiente

    Scenario: 50.1.4-EAC Rechazar audio del prestador en un chat pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        And que cargué y confirmé el audio "respuesta-prestador.webm" de 12 segundos
        When intento enviar únicamente el audio "respuesta-prestador.webm" en la conversación pendiente con la consumidora "Ana Pérez"
        Then el sistema rechaza el mensaje porque el prestador debe aceptar la solicitud de trabajo antes de responder
        And el sistema no asocia el audio a ningún mensaje

    Scenario: 50.1.5-EAC Contabilizar el audio para el límite de mensajes del consumidor en un chat pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que el consumidor "ana@example.com" ya alcanzó el límite de mensajes permitido en esa conversación pendiente
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio "mensaje-adicional.webm" de 8 segundos
        When intento enviar únicamente el audio "mensaje-adicional.webm" en la conversación pendiente con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque se alcanzó el límite de mensajes de la conversación pendiente
        And el sistema no asocia el audio a ningún mensaje

    Rule: El audio es una modalidad de mensaje exclusiva

    Scenario: 50.1.6-EAC Rechazar un mensaje que combina audio con texto
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio "ruido-bomba.webm" de 18 segundos
        When intento enviar el audio "ruido-bomba.webm" junto con el texto:
            """
            La bomba hace este ruido cuando abro la canilla.
            """
        Then el sistema rechaza el mensaje porque el audio debe enviarse sin texto ni imágenes
        And el sistema no asocia el audio a ningún mensaje

    Scenario: 50.1.7-EAC Rechazar un mensaje que combina audio con imágenes
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio "ruido-bomba.webm" de 18 segundos
        And que cargué y confirmé la imagen "bomba-de-agua.jpg"
        When intento enviar el audio "ruido-bomba.webm" junto con la imagen "bomba-de-agua.jpg"
        Then el sistema rechaza el mensaje porque el audio debe enviarse sin texto ni imágenes
        And el sistema no asocia el audio ni la imagen a ningún mensaje

    Scenario: 50.1.8-EAC Rechazar un mensaje que combina audio con texto e imágenes
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio "ruido-bomba.webm" de 18 segundos
        And que cargué y confirmé la imagen "bomba-de-agua.jpg"
        When intento enviar el audio "ruido-bomba.webm" junto con la imagen "bomba-de-agua.jpg" y el texto:
            """
            La bomba hace este ruido y además pierde agua por esta unión.
            """
        Then el sistema rechaza el mensaje porque el audio debe enviarse sin texto ni imágenes
        And el sistema no asocia el audio ni la imagen a ningún mensaje

    Rule: La contraparte puede recibir y consultar los audios del chat

    Scenario: 50.1.9-EAC La contraparte consulta un mensaje de audio
        Given que el consumidor "ana@example.com" envió el audio "ruido-bomba.webm" en el chat con el prestador "juan.plomero@example.com"
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When consulto el chat activo con el consumidor "ana@example.com"
        Then el detalle incluye el mensaje de audio "ruido-bomba.webm"
        And el detalle muestra la duración, el formato WebM y el codec Opus del audio
        And el sistema permite al prestador acceder al audio adjunto

    Scenario: 50.1.10-EAC La contraparte recibe en tiempo real un mensaje de audio
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio "ruido-bomba.webm" de 18 segundos
        When envío únicamente el audio "ruido-bomba.webm" en el chat con el prestador "Juan Gómez"
        Then el prestador "juan.plomero@example.com" recibe en tiempo real el mensaje de audio "ruido-bomba.webm"
        And el evento recibido incluye la duración y el acceso al audio

    Scenario: 50.1.11-EAC Mostrar un último mensaje de audio en el listado de conversaciones
        Given que estoy autenticado como consumidor "ana@example.com"
        And que tengo un chat activo con el prestador "juan.plomero@example.com" cuyo último mensaje es el audio "ruido-bomba.webm" de 18 segundos
        When consulto mis conversaciones
        Then el último mensaje de esa conversación se identifica como un mensaje de audio
        And el último mensaje informa una duración de 18 segundos

    Rule: Solo pueden enviarse audios confirmados y pertenecientes al remitente

    Scenario: 50.1.12-EAC Rechazar un audio que todavía no fue confirmado
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué pero no confirmé el audio "carga-incompleta.webm"
        When intento enviar únicamente el audio "carga-incompleta.webm" en el chat con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque el audio no está disponible
        And el sistema no asocia el audio a ningún mensaje

    Scenario: 50.1.13-EAC Rechazar un audio cargado por otro usuario
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que la consumidora "carla@example.com" cargó y confirmó el audio "audio-ajeno.webm"
        And que estoy autenticado como consumidor "ana@example.com"
        When intento enviar únicamente el audio "audio-ajeno.webm" en el chat con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque el audio no está disponible
        And el sistema no asocia el audio a ningún mensaje

    Scenario: 50.1.14-EAC Rechazar un archivo cargado para otra finalidad
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el archivo "grabacion.webm" para otra finalidad
        When intento enviar únicamente el archivo "grabacion.webm" como audio en el chat con el prestador "Juan Gómez"
        Then el sistema rechaza el mensaje porque el audio no está disponible
        And el sistema no asocia el archivo a ningún mensaje

    Rule: Los audios son privados para los participantes del chat

    Scenario: 50.1.15-EAC Rechazar el acceso de un consumidor ajeno al audio
        Given que el consumidor "ana@example.com" envió el audio "ruido-bomba.webm" en el chat con el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "carla@example.com"
        When intento acceder al audio "ruido-bomba.webm" adjunto al mensaje
        Then el sistema me indica que no puedo acceder a ese audio

    Scenario: 50.1.16-EAC Rechazar el acceso de un prestador ajeno al audio
        Given que el consumidor "ana@example.com" envió el audio "ruido-bomba.webm" en el chat con el prestador "juan.plomero@example.com"
        And que estoy autenticado como prestador "pedro.plomero@example.com"
        When intento acceder al audio "ruido-bomba.webm" adjunto al mensaje
        Then el sistema me indica que no puedo acceder a ese audio

    Rule: Los audios deben usar el formato WebM con codec Opus, no superar 5 MiB y durar como máximo 300 segundos

    Scenario: 50.1.17-EAC Rechazar un formato de audio no soportado
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento cargar el audio "grabacion.m4a" con formato MP4 y codec AAC para un mensaje del chat
        Then el sistema rechaza la carga porque el audio no usa el formato WebM con codec Opus

    Scenario: 50.1.18-EAC Rechazar un audio que supera el tamaño máximo permitido
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento cargar un audio WebM con codec Opus de 6 MiB para un mensaje del chat
        Then el sistema rechaza la carga porque el audio supera el máximo de 5 MiB

    Scenario: 50.1.19-EAC Rechazar un audio que supera la duración máxima permitida
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué el audio WebM con codec Opus "grabacion-extensa.webm" de 301 segundos
        When intento confirmar el audio para un mensaje del chat
        Then el sistema rechaza la confirmación porque el audio supera el máximo de 300 segundos

    Scenario: 50.1.20-EAC Aceptar un audio que cumple exactamente los límites permitidos
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
        And que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé el audio WebM con codec Opus "audio-limite.webm" de 5 MiB y 300 segundos
        When envío únicamente el audio "audio-limite.webm" en el chat con el prestador "Juan Gómez"
        Then el sistema registra el mensaje de audio "audio-limite.webm" en el chat
