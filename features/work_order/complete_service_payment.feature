Feature: Completar el pago del servicio
    Como consumidor
    quiero completar el pago del servicio realizado
    para saldar el importe acordado

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"
        And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-07-06T10:00:00-03:00" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And que la fecha y hora actual del sistema es "2026-07-06T10:00:00-03:00"

    Rule: Completar el pago cobra únicamente el saldo acordado

        Scenario: 27.1-CPS Iniciar el checkout del saldo de una orden programada
            Given que estoy autenticado como consumidor "ana@example.com"
            When solicito completar el pago de la orden de trabajo
            Then el sistema entrega una URL para completar el checkout del saldo
            And la respuesta identifica el intento de pago del saldo en estado "checkout_ready"
            And la respuesta informa el siguiente desglose en pesos argentinos:
                | concepto                                  | monto    |
                | saldo del servicio                         | 80000.00 |
                | comisión de LoResuelvo pendiente           | 4000.00  |
                | total a pagar ahora                        | 84000.00 |
            And la orden de trabajo todavía no queda pagada por completo

    Rule: El pago aprobado y verificado completa el saldo de la orden

        Scenario: 27.2-CPS Marcar la orden como pagada después de aprobar el saldo
            Given que "ana@example.com" inició el checkout del saldo de la orden de trabajo
            When el sistema procesa una notificación válida de Mercado Pago y verifica un pago aprobado por "84000.00" pesos argentinos para ese saldo
            Then el intento de pago del saldo puede consultarse en estado "paid"
            And la orden de trabajo queda pagada por completo
            And el servicio todavía no queda confirmado como realizado

        Scenario Outline: 27.3-CPS Mantener la orden sin pagar cuando el pago resulta <resultado>
            Given que "ana@example.com" inició el checkout del saldo de la orden de trabajo
            When el sistema procesa una notificación válida de Mercado Pago y verifica un pago <resultado> para ese saldo
            Then el intento de pago del saldo puede consultarse en estado "<estado>"
            And la orden de trabajo todavía no queda pagada por completo
            And el servicio todavía no queda confirmado como realizado

            Examples:
                | resultado    | estado       |
                | en proceso   | processing   |
                | rechazado    | rejected     |

        Scenario: 27.4-CPS Permitir reintentar después de rechazar el pago del saldo
            Given que la orden de trabajo tiene un intento de pago del saldo rechazado
            And que estoy autenticado como consumidor "ana@example.com"
            When solicito nuevamente completar el pago de la orden de trabajo
            Then el sistema entrega una URL para completar un nuevo checkout del saldo
            And la respuesta identifica un nuevo intento de pago del saldo en estado "checkout_ready"
            And la orden de trabajo conserva el saldo pendiente

    Rule: Solo el consumidor de la orden puede completar el pago

        Scenario Outline: 27.5-CPS Rechazar el pago solicitado por <actor>
            Given que estoy autenticado como <rol> "<correo>"
            When intento completar el pago de la orden de trabajo
            Then el sistema deniega el pago del saldo
            And la orden de trabajo conserva el saldo pendiente

            Examples:
                | actor              | rol         | correo                     |
                | otro consumidor    | consumidor  | carla@example.com          |
                | el prestador       | prestador   | juan.plomero@example.com   |

        Scenario: 27.6-CPS Rechazar el pago sin una sesión válida
            Given que no tengo una sesión válida
            When intento completar el pago de la orden de trabajo
            Then el sistema deniega el acceso
            And la orden de trabajo conserva el saldo pendiente

    Rule: El saldo se paga a partir de la fecha y hora acordadas y una sola vez

        Scenario: 27.9-CPS Rechazar el pago antes de la fecha y hora programadas
            Given que la fecha y hora actual del sistema es "2026-07-06T09:59:59-03:00"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento completar el pago de la orden de trabajo
            Then el sistema rechaza el pago porque todavía no llegó la fecha y hora programadas
            And la orden de trabajo conserva el saldo pendiente
            And el sistema no registra una sesión de checkout del saldo

        Scenario: 27.10-CPS Evitar un segundo cobro después de completar el pago
            Given que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When solicito nuevamente completar el pago de la orden de trabajo
            Then el sistema informa que la orden de trabajo ya está pagada por completo
            And el sistema no registra un nuevo intento de pago del saldo
            And el sistema no registra una nueva sesión de checkout del saldo

    Rule: El checkout y la notificación externa del saldo son idempotentes

        Scenario: 27.11-CPS Evitar checkouts activos duplicados ante solicitudes concurrentes
            Given que estoy autenticado como consumidor "ana@example.com"
            When solicito concurrentemente dos veces completar el pago de la orden de trabajo
            Then el sistema conserva un único intento de pago activo para el saldo
            And el sistema conserva una única sesión de checkout activa para el saldo
            And ambas solicitudes obtienen la misma URL de checkout

        Scenario: 27.12-CPS Procesar una sola vez una notificación de pago duplicada
            Given que "ana@example.com" inició el checkout del saldo de la orden de trabajo
            When el sistema procesa dos veces la misma notificación válida de Mercado Pago y verifica el pago aprobado del saldo
            Then el sistema registra una única transacción para el pago externo
            And la orden de trabajo queda pagada por completo
            And el servicio todavía no queda confirmado como realizado
