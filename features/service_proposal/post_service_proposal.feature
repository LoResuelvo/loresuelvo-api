@wip
Feature: Crear propuesta de servicio
    Como prestador
    quiero crear una propuesta de servicio
    para poder brindar mis servicios a un consumidor

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
    
    @wip
    Scenario: 53.1-PSP El prestador crear una propuesta de servicio exitosa
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When envío una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema registra la propuesta de servicio
    
    Rule: Se envia la propuesta de servicio al consumidor ademas de crearla

    @wip
    Scenario: 53.2-PSP Envio de propuesta de servicio
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que el consumidor "ana@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When envío una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el consumidor "ana@example.com" recibe en tiempo real la notificación de propuesta de servicio

    Rule: El monto de la propuesta debe ser mayor a cero

    @wip
    Scenario: 53.3-PSP El prestador crear una propuesta de servicio con monto inválido
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "0.00" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque el monto es inválido
    
    @wip
    Scenario: 53.4-PSP El prestador crear una propuesta de servicio a un consumidor de otro chat
        Given existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And que existe un chat activo entre el consumidor "carla@example.com" y el prestador "pedro.plomero@example.com" con el mensaje inicial:
            """
            Hola Pedro, necesito arreglar una canilla del baño.
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "carla@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque no existe un chat activo con ese consumidor

    @wip
    Scenario: 53.5-PSP El prestador crear una propuesta de servicio a un consumidor con el que no tiene conversación
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque no existe un chat activo con ese consumidor

    @wip
    Scenario: 53.6-PSP El prestador crear una propuesta de servicio a un consumidor con conversacion pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque no existe un chat activo con ese consumidor

    Rule: La fecha y hora de la propuesta debe ser futura
    
    @wip
    Scenario: 53.7-PSP El prestador crear una propuesta de servicio con fecha y hora pasada
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-03T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque la fecha y hora debe ser futura
    
    @wip
    Scenario: 53.8-PSP El prestador crear una propuesta de servicio con falta de parametros
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" con falta de parámetros
        Then el sistema rechaza la propuesta de servicio porque faltan parámetros obligatorios
