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

    Scenario: 39.5 - EST Enviar hasta 5 mensajes por el chat pendiente
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

    Scenario: 39.6 - EST Escribir en chat sin aceptar solicitud de trabajo vinculada
        Given envío una solicitud de trabajo al prestador "Ana Pérez" con el título "Reparación de fuga en la cocina" y la descripción:
            """
            Hola Ana, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        When el prestador "Ana Pérez" intenta enviar un mensaje en el chat pendiente con el consumidor "Diego" sin aceptar la solicitud de trabajo vinculada
        Then el sistema muestra un mensaje de error indicando que no se puede enviar mensajes en el chat pendiente sin aceptar la solicitud de trabajo vinculada
