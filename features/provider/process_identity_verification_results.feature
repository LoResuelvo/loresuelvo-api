Feature: Procesar resultados de verificación de identidad
    Como prestador que inició una verificación
    quiero que LoResuelvo procese sus resultados
    para conocer el estado actual de mi identidad

    Background:
        Given que existe el rubro "Plomería"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que el prestador "juan.plomero@example.com" inició una verificación de identidad

    Rule: El estado local debe reflejar el último resultado auténtico

        Scenario: 58.1.1 Registrar que la verificación está en progreso
            When el verificador informa de forma auténtica que la sesión está "in_progress"
            Then "juan.plomero@example.com" consulta en su perfil el estado "in_progress"

        Scenario: 58.1.2 Registrar que la verificación espera una acción del usuario
            When el verificador informa de forma auténtica que la sesión está "awaiting_user"
            Then "juan.plomero@example.com" consulta en su perfil el estado "awaiting_user"

        @wip
        Scenario: 58.1.3 Registrar que la verificación requiere revisión manual
            When el verificador informa de forma auténtica que la sesión está "in_review"
            Then "juan.plomero@example.com" consulta en su perfil el estado "in_review"

        @wip
        Scenario: 58.1.4 Aprobar la identidad del prestador
            Given que la fecha y hora actual del sistema es "2026-09-01T12:00:00Z"
            When el verificador informa de forma auténtica que la sesión está "approved"
            Then "juan.plomero@example.com" consulta en su perfil el estado "approved"
            And la fecha de verificación de "juan.plomero@example.com" es "2026-09-01T12:00:00Z"

        @wip
        Scenario: 58.1.5 Registrar una verificación rechazada
            When el verificador informa de forma auténtica que la sesión está "declined" por el riesgo "DOCUMENT_EXPIRED"
            Then "juan.plomero@example.com" consulta en su perfil el estado "declined"
            And el sistema conserva solamente el código de riesgo "DOCUMENT_EXPIRED"

        @wip
        Scenario: 58.1.6 Registrar una solicitud de reenvío
            When el verificador informa de forma auténtica que la sesión está "resubmitted"
            Then "juan.plomero@example.com" consulta en su perfil el estado "resubmitted"

        @wip
        Scenario: 58.1.7 Registrar una verificación abandonada
            When el verificador informa de forma auténtica que la sesión está "abandoned"
            Then "juan.plomero@example.com" consulta en su perfil el estado "abandoned"

        @wip
        Scenario: 58.1.8 Registrar una sesión expirada
            When el verificador informa de forma auténtica que la sesión está "expired"
            Then "juan.plomero@example.com" consulta en su perfil el estado "expired"

        @wip
        Scenario: 58.1.9 Registrar la expiración de una identidad aprobada
            Given que la identidad de "juan.plomero@example.com" está aprobada
            When el verificador informa de forma auténtica que la sesión está "kyc_expired"
            Then "juan.plomero@example.com" consulta en su perfil el estado "kyc_expired"
            And "juan.plomero@example.com" ya no figura como prestador verificado

    Rule: Las notificaciones deben ser auténticas, idempotentes y ordenadas

        @wip
        Scenario: 58.1.10 Rechazar una notificación con firma inválida
            When se recibe un resultado "approved" con una firma de verificación inválida
            Then el sistema rechaza la notificación
            And la verificación de "juan.plomero@example.com" permanece en estado "not_started"

        @wip
        Scenario: 58.1.11 Procesar una única vez una notificación repetida
            When el verificador envía dos veces el mismo resultado auténtico "approved"
            Then ambas entregas son aceptadas
            And la identidad de "juan.plomero@example.com" queda aprobada
            And el resultado queda registrado una sola vez

        @wip
        Scenario: 58.1.12 Ignorar un resultado anterior al estado actual
            Given que la identidad de "juan.plomero@example.com" fue aprobada a las "2026-09-01T12:00:00Z"
            When llega un resultado auténtico "in_progress" ocurrido a las "2026-09-01T11:59:00Z"
            Then "juan.plomero@example.com" conserva el estado "approved"
