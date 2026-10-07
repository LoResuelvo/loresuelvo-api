Feature: Iniciar la revisión administrativa de un reclamo
    Como operador autorizado de soporte y mediación
    quiero iniciar expresamente la revisión de un expediente
    para dejar constancia de cuándo y quién comenzó a investigarlo

    Background:
        Given que existen el consumidor registrado "ana@example.com" y el prestador registrado "juan@example.com"
        And que ambos participan en la solicitud de trabajo "S1"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: Sólo un expediente abierto puede iniciar revisión

        @wip
        Scenario: 68.14-RC Iniciar revisión sin alterar el testimonio ni la operación
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And que "ana@example.com" presentó el reclamo abierto "C1" sobre "S1" con motivo "non_compliance" y testimonio "El servicio acordado no se realizó"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When inicio la revisión administrativa del reclamo "C1" con la clave "550e8400-e29b-41d4-a716-446655440017" y la correlación "claim-review-1"
            Then el sistema responde con estado 200 y el reclamo queda en estado "in_review"
            And la actuación registra al operador autenticado y el instante "2026-09-25T12:00:00-03:00" del servidor
            And el motivo, testimonio, referencias, evidencias y fechas originales del reclamo permanecen sin cambios
            And queda registrada una sola actuación de revisión
            And queda persistido un evento de ejecución exitoso del reclamo "C1" por "operador@example.com" en el instante "2026-09-25T12:00:00-03:00" y con la correlación "claim-review-1"
            And el evento sólo registra el cambio de estado de "open" a "in_review", sin motivo ni testimonio
            And la operación, sus propuestas, órdenes y pagos permanecen sin cambios
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        @wip
        Scenario Outline: 68.15-RC Rechazar el inicio de revisión cuando el expediente no está abierto
            Given que "ana@example.com" tiene el reclamo "C1" en estado "<estado>"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When intento iniciar la revisión administrativa del reclamo "C1" con una clave nueva
            Then el sistema responde con estado 409
            And el estado y las actuaciones previas del reclamo "C1" permanecen sin cambios

            Examples:
                | estado    |
                | in_review |
                | resolved  |
                | dismissed |

    Rule: La clave de idempotencia conserva la actuación original

        @wip
        Scenario: 68.16-RC Recuperar la revisión original después de que el expediente finaliza
            Given que la fecha y hora actual del sistema es "2026-09-24T08:00:00-03:00"
            And que "ana@example.com" tiene el reclamo "C1" abierto sobre la operación "S1"
            And que el administrador "operador@example.com" inició la revisión de "C1" con la clave "550e8400-e29b-41d4-a716-446655440019" en el instante "2026-09-24T10:00:00-03:00"
            And el administrador "operador@example.com" finalizó luego "C1" con el dictamen "consumer_favor" y fundamentación "Se verificó el incumplimiento acordado" en el instante "2026-09-24T12:00:00-03:00"
            And que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When repito el inicio de revisión de "C1" con la misma clave y el mismo contenido
            Then el sistema responde con estado 200 y el reclamo conserva el estado final "resolved"
            And la respuesta recupera la actuación original con el operador y el instante originales
            And no se duplica la actuación de revisión ni su auditoría

        @wip
        Scenario Outline: 68.17-RC Rechazar una clave ausente o que no es un UUID
            Given que "ana@example.com" tiene el reclamo abierto "C1"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When intento iniciar la revisión administrativa de "C1" con <clave>
            Then el sistema responde con estado 400
            And el reclamo "C1" permanece abierto sin actuaciones administrativas nuevas

            Examples:
                | clave                 |
                | una clave ausente     |
                | la clave "no-es-uuid" |

    Rule: El permiso específico autoriza el inicio de revisión

        @wip
        Scenario: 68.18-RC Rechazar el inicio sin permiso de escritura de reclamos
            Given que "ana@example.com" tiene el reclamo abierto "C1"
            And estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_claims"
            When intento iniciar la revisión administrativa del reclamo "C1" con una clave nueva
            Then el sistema responde con estado 403
            And el reclamo "C1" permanece abierto sin actuaciones administrativas nuevas
