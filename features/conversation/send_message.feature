Feature: Envío de mensaje
    Como consumidor
    quiero intercambiar mensajes de texto con un prestador
    para coordinar detalles del servicio solicitado dentro de la plataforma

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """

    Scenario: 01-EM Consumidor envía un mensaje en una conversación existente
        Given que estoy autenticado como consumidor "ana@example.com"
        When envío un mensaje en la conversación pendiente con el prestador "Juan Gómez":
            """
            ¿El jueves por la mañana te queda cómodo para pasar a revisar el problema?
            """
        Then el sistema registra el mensaje en la conversación
        And el último mensaje de la conversación fue enviado por "Ana Pérez" con el contenido:
            """
            ¿El jueves por la mañana te queda cómodo para pasar a revisar el problema?
            """

    Scenario: 02-EM Prestador responde en una conversación existente
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When envío un mensaje en la conversación pendiente con el consumidor "Ana Pérez":
            """
            Sí, puedo pasar el jueves a las 10. ¿Te queda cómodo?
            """
        Then el sistema registra el mensaje en la conversación
        And el último mensaje de la conversación fue enviado por "Juan Gómez" con el contenido:
            """
            Sí, puedo pasar el jueves a las 10. ¿Te queda cómodo?
            """

    Rule: Solo los participantes pueden enviar mensajes en una conversación

    Scenario: 03-EM Rechazar mensaje de consumidor que no participa en la conversación
        Given que estoy autenticado como consumidor "carla@example.com"
        When intento enviar un mensaje en la conversación pendiente con el prestador "Juan Gómez":
            """
            Hola, quisiera sumarme a esta conversación.
            """
        Then el sistema me indica que no puedo acceder a esa conversación

    Scenario: 04-EM Rechazar mensaje de prestador que no participa en la conversación
        Given que estoy autenticado como prestador "pedro.plomero@example.com"
        When intento enviar un mensaje en la conversación pendiente con el consumidor "Ana Pérez":
            """
            Puedo ayudarte con ese trabajo.
            """
        Then el sistema me indica que no puedo acceder a esa conversación

    Scenario: 05-EM Rechazar mensaje sin sesión válida
        Given que no tengo una sesión válida
        When intento enviar un mensaje en la conversación pendiente con el prestador "Juan Gómez":
            """
            ¿Seguimos coordinando por acá?
            """
        Then el sistema deniega el acceso

    Rule: El mensaje debe pertenecer a una conversación existente

    Scenario: 06-EM Rechazar mensaje en conversación inexistente
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento enviar un mensaje en una conversación inexistente:
            """
            ¿Podemos coordinar una visita?
            """
        Then el sistema me indica que la conversación no existe

    Rule: El contenido del mensaje debe ser válido

    Scenario: 07-EM Rechazar mensaje sin contenido
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento enviar un mensaje en la conversación pendiente con el prestador "Juan Gómez" sin contenido
        Then el sistema me indica que el mensaje es obligatorio

    Scenario: 08-EM Rechazar mensaje con contenido vacío
        Given que estoy autenticado como consumidor "ana@example.com"
        When intento enviar un mensaje en la conversación pendiente con el prestador "Juan Gómez" con el mensaje "   "
        Then el sistema me indica que el mensaje es obligatorio
