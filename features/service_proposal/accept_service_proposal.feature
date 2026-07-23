Feature: Contratar un servicio mediante el pago de una seña
    Como consumidor
    quiero pagar la seña de una propuesta de servicio
    para confirmar la contratación y generar el turno acordado

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"

    Rule: Solicitar la confirmación inicia el checkout sin aceptar todavía la propuesta

        Scenario: 21.1-CSP Iniciar el checkout de la seña de una propuesta pendiente
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-07-06T10:00:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como consumidor "ana@example.com"
            When solicito pagar la seña de la propuesta de servicio pendiente
            Then el sistema entrega una URL para completar el checkout de la seña
            And la respuesta identifica el intento de pago en estado "checkout_ready"
            And la respuesta informa el siguiente desglose en pesos argentinos:
                | concepto                              | monto     |
                | precio total del servicio              | 100000.00 |
                | seña del prestador                     | 20000.00  |
                | comisión total de LoResuelvo           | 5000.00   |
                | comisión de LoResuelvo cobrada ahora   | 1000.00   |
                | total a pagar ahora                    | 21000.00  |
                | saldo total a pagar más adelante       | 84000.00  |
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

    Rule: La vigencia de la URL de checkout es de treinta minutos o hasta el límite de pago de la propuesta, lo que ocurra primero

        @wip
        Scenario Outline: 21.2-CSP Establecer la vigencia del checkout cuando <caso>
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" programada para "2026-07-06T10:00:00-03:00"
            And que la fecha y hora actual del sistema es "<fecha y hora actual>"
            And que estoy autenticado como consumidor "ana@example.com"
            When solicito pagar la seña de la propuesta de servicio pendiente
            Then la respuesta informa que la sesión de checkout vence en "<fecha y hora de vencimiento>"

            Examples:
                | caso                                  | fecha y hora actual             | fecha y hora de vencimiento      |
                | hay treinta minutos disponibles       | 2026-07-04T10:00:00-03:00       | 2026-07-04T10:30:00-03:00        |
                | el límite de pago ocurre antes        | 2026-07-05T09:45:00-03:00       | 2026-07-05T10:00:00-03:00        |

    Rule: La contratación se confirma únicamente con un pago aprobado y verificado

        @wip
        Scenario: 21.4-CSP Aceptar la propuesta y generar una única orden con la seña aprobada
            Given que "ana@example.com" inició el checkout de la seña de una propuesta pendiente de "juan.plomero@example.com"
            When el sistema procesa una notificación válida de Mercado Pago y verifica un pago aprobado por "21000.00" pesos argentinos para esa seña
            Then el intento de pago puede consultarse en estado "paid"
            And la propuesta de servicio queda aceptada
            And el sistema registra una única orden de trabajo programada
            And la orden de trabajo queda vinculada a la propuesta aceptada
            And la orden de trabajo conserva el consumidor, el prestador, el precio del servicio, la fecha y hora y la descripción acordados

        @wip
        Scenario: 21.5-CSP Notificar al prestador después de aprobar la seña
            Given que "ana@example.com" inició el checkout de la seña de una propuesta pendiente de "juan.plomero@example.com"
            And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
            When el sistema procesa una notificación válida de Mercado Pago y verifica un pago aprobado para esa seña
            Then el prestador "juan.plomero@example.com" recibe en tiempo real la notificación de propuesta de servicio aceptada

        @wip
        Scenario: 21.6-CSP Mantener pendiente la propuesta mientras el pago está en proceso
            Given que "ana@example.com" inició el checkout de la seña de una propuesta pendiente de "juan.plomero@example.com"
            When el sistema procesa una notificación válida de Mercado Pago y verifica un pago en proceso para esa seña
            Then el intento de pago puede consultarse en estado "processing"
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

        @wip
        Scenario: 21.7-CSP Permitir reintentar después de un pago rechazado
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" con un intento de pago rechazado
            And que estoy autenticado como consumidor "ana@example.com"
            When "ana@example.com" solicita nuevamente pagar la seña de la propuesta
            Then el sistema entrega una URL para completar un nuevo checkout
            And la respuesta identifica un nuevo intento de pago en estado "checkout_ready"
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta
            
    Rule: Solo el consumidor destinatario puede iniciar el pago

        @wip
        Scenario: 21.10-CSP Rechazar el checkout solicitado por otro consumidor
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como consumidor "carla@example.com"
            When intento pagar la seña de la propuesta de servicio pendiente de "ana@example.com"
            Then el sistema deniega el pago de la seña
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

        @wip
        Scenario: 21.11-CSP Rechazar el checkout solicitado por el prestador
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When intento pagar la seña de la propuesta de servicio pendiente
            Then el sistema deniega el pago de la seña
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

        @wip
        Scenario: 21.12-CSP Rechazar el checkout sin una sesión válida
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que no tengo una sesión válida
            When intento pagar la seña de la propuesta de servicio pendiente
            Then el sistema deniega el acceso
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

    Rule: Solo las propuestas pendientes y dentro del límite de pago admiten checkout

        @wip
        Scenario: 21.13-CSP Rechazar el checkout de una propuesta ya aceptada
            Given que existe una propuesta de servicio aceptada de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento pagar la seña de la propuesta de servicio aceptada
            Then el sistema rechaza pagar una propuesta de servicio ya aceptada
            And el sistema conserva una única orden de trabajo para la propuesta

        @wip
        Scenario: 21.14-CSP Rechazar el checkout de una propuesta rechazada
            Given que existe una propuesta de servicio rechazada de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento pagar la seña de la propuesta de servicio rechazada
            Then el sistema rechaza pagar una propuesta de servicio rechazada
            And el sistema no registra una orden de trabajo para la propuesta

        @wip
        Scenario: 21.15-CSP Rechazar el checkout al alcanzar el límite de pago
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" programada para "2026-07-06T10:00:00-03:00"
            And que la fecha y hora actual del sistema es "2026-07-05T10:00:00-03:00"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento pagar la seña de la propuesta de servicio pendiente
            Then el sistema rechaza el pago porque finalizó el plazo para confirmar la contratación
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

    Rule: Crear el checkout es idempotente ante solicitudes repetidas o concurrentes

        Background:
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "100000.00" programada para "2026-07-06T10:00:00-03:00"
            And que estoy autenticado como consumidor "ana@example.com"

        @wip
        Scenario: 21.18-CSP Evitar sesiones activas duplicadas
            When solicito concurrentemente dos veces pagar la seña de la propuesta
            Then el sistema conserva un único intento de pago activo para la propuesta
            And el sistema conserva una única sesión de checkout activa para la propuesta
            And ambas solicitudes obtienen la misma URL de checkout

    Rule: Las notificaciones externas no pueden duplicar la contratación

        Background:
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "100000.00" programada para "2026-07-06T10:00:00-03:00"

        @wip
        Scenario: 21.20-CSP Procesar una sola vez una notificación duplicada
            Given que inicié el checkout de la seña de la propuesta
            When el sistema procesa dos veces la misma notificación válida de Mercado Pago y verifica el pago aprobado
            Then el sistema registra una única transacción para el pago externo
            And la propuesta de servicio queda aceptada
            And el sistema registra una única orden de trabajo para la propuesta

    Rule: Un pago aprobado solo confirma la contratación si coincide con la obligación interna

        Background:
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "100000.00" programada para "2026-07-06T10:00:00-03:00"
            And que inicié el checkout de la seña de la propuesta

    Rule: Los pagos aprobados fuera de las reglas de contratación requieren devolución manual

        Background:
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "100000.00" programada para "2026-07-06T10:00:00-03:00"

        @wip
        Scenario: 21.26-CSP Requerir devolución si el pago se aprueba después del límite
            Given que inicié el checkout de la seña de la propuesta
            And que el límite para pagar la seña era "2026-07-05T10:00:00-03:00"
            When el sistema procesa una notificación válida de Mercado Pago y verifica que el pago se aprobó en "2026-07-05T10:00:01-03:00"
            Then el sistema registra un incidente de pago "refund_required"
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

        @wip
        Scenario: 21.26.1-CSP Confirmar un pago aprobado a tiempo aunque la notificación llegue después del límite
            Given que inicié el checkout de la seña de la propuesta
            And que el límite para pagar la seña era "2026-07-05T10:00:00-03:00"
            And que la fecha y hora actual del sistema es "2026-07-05T10:05:00-03:00"
            When el sistema procesa una notificación válida de Mercado Pago y verifica que el pago se aprobó en "2026-07-05T09:59:59-03:00"
            Then el intento de pago puede consultarse en estado "paid"
            And la propuesta de servicio queda aceptada
            And el sistema registra una única orden de trabajo para la propuesta

        @wip
        Scenario: 21.27-CSP Requerir devolución para un segundo pago aprobado
            Given que un primer pago aprobado ya confirmó la propuesta y generó su orden de trabajo
            When el sistema procesa una notificación válida de Mercado Pago y verifica un segundo pago aprobado para la misma seña
            Then el sistema registra un incidente de pago "refund_required" para el segundo pago
            And la propuesta de servicio permanece aceptada
            And el sistema conserva una única orden de trabajo para la propuesta

    Rule: La cuenta conectada del prestador debe permitir crear el checkout

        Background:
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "100000.00" programada para "2026-07-06T10:00:00-03:00"
            And que estoy autenticado como consumidor "ana@example.com"

        @wip
        Scenario: 21.28-CSP Renovar una credencial vencida antes de crear el checkout
            Given que la credencial de Mercado Pago de "juan.plomero@example.com" venció
            And que su autorización permite renovarla
            When solicito pagar la seña de la propuesta
            Then el sistema entrega una URL HTTPS de Checkout Pro
            And la cuenta de pago del prestador permanece conectada

        @wip
        Scenario: 21.29-CSP Solicitar una nueva autorización si la credencial no puede renovarse
            Given que la credencial de Mercado Pago de "juan.plomero@example.com" venció
            And que Mercado Pago rechaza su renovación
            When solicito pagar la seña de la propuesta
            Then el sistema informa que la cuenta de pago del prestador debe volver a autorizarse
            And la cuenta de pago del prestador queda en estado "reauthorization_required"
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una sesión de checkout activa

    Rule: Una reversión posterior hace visible el problema sobre la orden

        Background:
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "100000.00" programada para "2026-07-06T10:00:00-03:00"

        @wip
        Scenario Outline: 21.30-CSP Marcar la orden ante <reversión> del pago
            Given que el pago aprobado de la seña confirmó la propuesta y generó su orden de trabajo
            When el sistema procesa una notificación válida de Mercado Pago y verifica <reversión> del pago de la seña
            Then la orden de trabajo queda marcada como "payment_issue"

            Examples:
                | reversión       |
                | una devolución  |
                | un contracargo  |
