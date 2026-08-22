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
        And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"
    
    Scenario: 53.1-PSP El prestador crear una propuesta de servicio exitosa
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When envío una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema registra la propuesta de servicio
    
    Rule: Se envia la propuesta de servicio al consumidor ademas de crearla

    Scenario: 53.2.1-PSP Envio de propuesta de servicio con duración estimada
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que el consumidor "ana@example.com" está disponible para recibir mensajes en tiempo real
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When envío una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el consumidor "ana@example.com" recibe en tiempo real la notificación de propuesta de servicio
        And la notificación de propuesta incluye una duración estimada de "60" minutos

    @wip
    Scenario: 53.2.2-PSP Registrar y devolver una duración estimada válida
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When envío una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con una duración estimada de "90" minutos con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema registra la propuesta de servicio
        And la propuesta de servicio informa una duración estimada de "90" minutos

    @wip
    Scenario: 53.2.3-PSP Rechazar una propuesta sin duración estimada
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" sin indicar la duración estimada con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque la duración estimada es obligatoria

    @wip
    Scenario: 53.2.4-PSP Rechazar una duración estimada menor a quince minutos
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con una duración estimada de "14" minutos con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque la duración estimada está fuera de rango

    @wip
    Scenario: 53.2.5-PSP Rechazar una duración estimada mayor a veinticuatro horas
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con una duración estimada de "1441" minutos con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque la duración estimada está fuera de rango

    Rule: El monto de la propuesta debe ser mayor a cero

    Scenario: 53.3-PSP El prestador crear una propuesta de servicio con monto inválido
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "0.00" para la fecha y hora "2026-07-06T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque el monto es inválido
    
    Scenario: 53.4-PSP El prestador crear una propuesta de servicio a un consumidor de otro chat
        Given existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And que existe un chat activo entre el consumidor "carla@example.com" y el prestador "pedro.plomero@example.com" con el mensaje inicial:
            """
            Hola Pedro, necesito arreglar una canilla del baño.
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "carla@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque no existe un chat activo con ese consumidor

    Scenario: 53.5-PSP El prestador crear una propuesta de servicio a un consumidor con el que no tiene conversación
        Given que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque no existe un chat activo con ese consumidor

    Scenario: 53.6-PSP El prestador crear una propuesta de servicio a un consumidor con conversacion pendiente
        Given que existe una conversación pendiente entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        Then el sistema rechaza la propuesta de servicio porque no existe un chat activo con ese consumidor

    Rule: La fecha y hora de la propuesta debe ser futura
    
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
    
    Scenario: 53.8-PSP El prestador crear una propuesta de servicio con falta de parametros
        Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
            """
            Hola Juan, necesito reparar una pérdida de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When intento enviar una propuesta de servicio al consumidor "ana@example.com" con falta de parámetros
        Then el sistema rechaza la propuesta de servicio porque faltan parámetros obligatorios

    Rule: Se congelan los términos económicos al crear la propuesta aplicando una seña del veinte por ciento y una comisión total fija de cinco mil pesos

        Background:
            Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"

        Scenario: 53.9-PSP Calcular la seña y la comisión de la propuesta
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When envío una propuesta con precio total de servicio de "100000.00" pesos para la fecha y hora "2026-07-06T10:00:00-03:00"
            Then el sistema registra la propuesta de servicio
            And la propuesta conserva el siguiente desglose en pesos argentinos:
                | concepto                              | monto     |
                | precio total del servicio              | 100000.00 |
                | seña del prestador                     | 20000.00  |
                | saldo del servicio                     | 80000.00  |
                | comisión total de LoResuelvo           | 5000.00   |
                | comisión de LoResuelvo cobrada ahora   | 1000.00   |
                | comisión de LoResuelvo pendiente       | 4000.00   |
                | total a pagar ahora                    | 21000.00  |
                | saldo total a pagar más adelante       | 84000.00  |
                | total de la contratación               | 105000.00 |

    Rule: La fecha programada debe dejar tiempo para pagar al menos un día antes

        Background:
            Given que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"

        Scenario Outline: 53.12-PSP Rechazar una propuesta que <caso>
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When intento enviar una propuesta con precio total de servicio de "100000.00" pesos para la fecha y hora "<fecha y hora programada>"
            Then el sistema rechaza la propuesta porque no deja tiempo para pagar al menos un día antes

            Examples:
                | caso                                      | fecha y hora programada          |
                | está programada exactamente a veinticuatro horas | 2026-07-05T10:00:00-03:00   |
                | está programada a menos de veinticuatro horas    | 2026-07-05T09:59:59-03:00   |

        Scenario: 53.14-PSP Admitir una propuesta que deja más de veinticuatro horas
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When envío una propuesta con precio total de servicio de "100000.00" pesos para la fecha y hora "2026-07-05T11:00:00-03:00"
            Then el sistema registra la propuesta de servicio
            And el límite para pagar la seña queda fijado en "2026-07-04T11:00:00-03:00"
