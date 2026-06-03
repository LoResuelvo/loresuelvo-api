@wip
Feature: Chat en tiempo real
    Como participante de un chat
    quiero recibir los mensajes nuevos en el momento en que son enviados
    para coordinar el servicio sin tener que actualizar la conversacion manualmente

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    Scenario: 01-TR La contraparte disponible recibe un mensaje de chat en tiempo real
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como consumidor "ana@example.com"
        When envío un mensaje en el chat con el prestador "Juan Gómez":
            """
            ¿El jueves por la mañana te queda cómodo para pasar a revisar el problema?
            """
        Then el sistema registra el mensaje en la conversación
        And el prestador "juan.plomero@example.com" recibe en tiempo real el mensaje:
            """
            ¿El jueves por la mañana te queda cómodo para pasar a revisar el problema?
            """

    Scenario: 02-TR El mensaje se registra aunque la contraparte no esté disponible
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como consumidor "ana@example.com"
        When envío un mensaje en el chat con el prestador "Juan Gómez":
            """
            Te dejo mi disponibilidad para coordinar la visita.
            """
        Then el sistema registra el mensaje en la conversación
        And el último mensaje de la conversación fue enviado por "Ana Pérez" con el contenido:
            """
            Te dejo mi disponibilidad para coordinar la visita.
            """

    Scenario: 03-TR Un usuario ajeno no recibe mensajes de un chat en el que no participa
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que el prestador "pedro.plomero@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como consumidor "ana@example.com"
        When envío un mensaje en el chat con el prestador "Juan Gómez":
            """
            Confirmo que seguimos coordinando por este chat.
            """
        Then el sistema registra el mensaje en la conversación
        And el prestador "pedro.plomero@example.com" no recibe mensajes en tiempo real

    Scenario: 04-TR Un mensaje rechazado no se entrega en tiempo real
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que el consumidor "ana@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar un mensaje en la conversación pendiente con el consumidor "Ana Pérez":
            """
            Sí, puedo pasar el jueves a las 10. ¿Te queda cómodo?
            """
        Then el sistema muestra un mensaje de error indicando que no se puede enviar mensajes en el chat pendiente sin aceptar la solicitud de trabajo vinculada
        And el consumidor "ana@example.com" no recibe mensajes en tiempo real
