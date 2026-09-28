Feature: Consultar un diagnóstico administrativo acotado de prestador
    Como administrador autorizado
    quiero consultar evidencia operativa persistida de un prestador
    para comprender su situación sin convertir señales parciales en decisiones automáticas

    Background:
        Given que existe el rubro "Plomería"
        And que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14"
        And que existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y la foto de perfil cargada
        And que "juan@example.com" cubre la zona "Comuna 6"
        And que "juan@example.com" cubre la zona "Comuna 14"
        And que existen los siguientes consumidores registrados:
            | correo              | nombre  | apellido |
            | ana@example.com     | Ana     | Pérez    |
            | beatriz@example.com | Beatriz | Suárez   |
        And que existe un administrador provisionado con correo "soporte@example.com", nombre "Sofía" y apellido "López"

    Rule: El diagnóstico distingue evidencia positiva, negativa y desconocida sin resumirla en un veredicto
        Background:
            Given que estoy autenticado como administrador "soporte@example.com" con el permiso "read:providers"

        Scenario: 61.1.1-PD Entregar controles concretos basados en evidencia persistida
            Given que la fecha y hora actual del sistema es "2026-09-28T00:00:00Z"
            And que la identidad de "juan@example.com" fue aprobada el "2026-09-18T15:30:00Z"
            And que existe una cuenta de Mercado Pago "mp-juan" vinculada al prestador "juan@example.com" con expiración persistida "2026-09-29T00:00:00Z"
            And que "juan@example.com" tiene Google Calendar conectado desde "2026-09-18T15:45:00Z"
            When consulto el diagnóstico administrativo del prestador "juan@example.com" con correlación "provider-diagnostic-1"
            Then el sistema responde con estado 200
            And el diagnóstico identifica al prestador por su ID, rol, nombre, apellido, correo y fecha de registro, e incluye la URL pública de la foto de perfil persistida
            And el diagnóstico incluye el rubro "Plomería" y las zonas "Comuna 6" y "Comuna 14" con sus identificadores y disponibilidad habilitada
            And los controles informan exactamente:
                | control                    | resultado | reason_code                 | evidence_on          |
                | identity_verification      | pass      | identity_approved           | 2026-09-18T15:30:00Z |
                | payment_account_connection | pass      | payment_account_connected   | valor persistido de connected_on |
                | payment_token_expiry       | pass      | payment_token_expiry_future | 2026-09-29T00:00:00Z |
                | calendar_connection        | pass      | calendar_connected          | 2026-09-18T15:45:00Z |
            And la evidencia de conexión de pago coincide con el connected_on persistido de "mp-juan", sin fijar su fecha en el fixture
            And las fechas de evidencia sólo aparecen cuando existe un instante persistido para ese control
            And la expiración informada corresponde a la fecha guardada en la cuenta
            And la respuesta no contiene un veredicto booleano general de aptitud, credenciales, tokens, secretos, sesiones privadas, documentos ni códigos de riesgo
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"
            And antes de entregar el diagnóstico queda preparado exactamente un evento de acceso al recurso "provider" con el ID persistido de "juan@example.com", el operador "soporte@example.com" y la correlación "provider-diagnostic-1"
            And el resultado del evento de acceso es "prepared", no "succeeded", y no contiene datos del diagnóstico ni una justificación manual

        @wip
        Scenario: 61.1.2-PD Diferenciar evidencia ausente de una falla de lectura
            Given que "juan@example.com" no tiene sesiones de verificación de identidad
            And que no existe una cuenta de Mercado Pago ni una conexión de Google Calendar para "juan@example.com"
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And identidad informa estado "unverified", resultado "warning", reason_code "identity_no_session" y evidencia nula
            And conexión de pago informa estado "disconnected", resultado "warning", reason_code "payment_account_not_connected" y evidencia nula
            And expiración del token informa resultado "unknown", reason_code "payment_token_expiry_unavailable" y evidencia nula
            And Calendar informa estado "disconnected", resultado "warning", reason_code "calendar_not_connected" y evidencia nula
            And reputación informa cero reviews y promedio 0

        Scenario: 61.1.3-PD Fallar cerrado cuando no se puede leer una fuente requerida
            Given que falla la lectura de una fuente requerida del diagnóstico del prestador "juan@example.com"
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 500
            And no se entrega un diagnóstico parcial ni se presenta el fallo como conexión ausente
            And no se registra ningún evento de auditoría para esta solicitud

    Rule: La conexión de pago, la expiración guardada y las reglas de cobertura se informan por separado
        Background:
            Given que estoy autenticado como administrador "soporte@example.com" con el permiso "read:providers"

        Scenario: 61.1.4-PD Informar una expiración persistida ya vencida
            Given que la fecha y hora actual del sistema es "2026-09-28T00:00:00Z"
            And que existe una cuenta de Mercado Pago "mp-juan" vinculada al prestador "juan@example.com" con expiración persistida "2026-09-27T23:00:00Z"
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And el control de conexión informa "pass" con reason_code "payment_account_connected"
            And el control "payment_token_expiry" informa "warning" con reason_code "payment_token_expiry_elapsed" y evidencia "2026-09-27T23:00:00Z"
            And la expiración informada es exclusivamente el dato persistido en la cuenta

        @wip
        Scenario: 61.1.5-PD No afirmar que la falta de pago conectado impide recibir solicitudes
            Given que no existe una cuenta de Mercado Pago para "juan@example.com"
            And que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor      | prestador        | creada               | estado  |
                | S1        | ana@example.com | juan@example.com | 2026-09-20T12:00:00Z | pending |
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And el control de conexión de pagos informa "warning" con reason_code "payment_account_not_connected"
            And el control de expiración informa "unknown" con reason_code "payment_token_expiry_unavailable"
            And la actividad incluye la solicitud "S1" con estado "pending"
            And la respuesta no incluye un campo ni un control que afirme que el prestador no puede recibir solicitudes

        Scenario: 61.1.6-PD Informar la disponibilidad actual de las zonas de cobertura
            Given que la zona de cobertura "Comuna 14" se deshabilita después del registro de "juan@example.com"
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And el diagnóstico incluye "Comuna 14" con su disponibilidad deshabilitada y conserva "Comuna 6" como habilitada

    Rule: Calendar distingue su estado de conexión de la sincronización por participante
        Background:
            Given que estoy autenticado como administrador "soporte@example.com" con el permiso "read:providers"

        Scenario: 61.1.7-PD No confundir una conexión de Calendar que requiere autorización con desconexión
            Given que la conexión de Google Calendar de "juan@example.com" está en estado "action_required" desde "2026-09-27T10:00:00Z"
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And Calendar informa el estado "action_required", resultado "warning", reason_code "calendar_authorization_required" y evidencia "2026-09-27T10:00:00Z"
            And la conexión no se informa como "disconnected" ni como sincronización confirmada de órdenes

        @wip
        Scenario: 61.1.8-PD Distinguir conexión Calendar de sincronización individual de órdenes
            Given que la fecha y hora actual del sistema es "2026-09-26T00:00:00Z"
            And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan@example.com"
            And que "juan@example.com" tiene Google Calendar conectado desde "2026-09-18T15:45:00Z"
            And que existe una orden de trabajo futura para "ana@example.com" y "juan@example.com" el "2026-10-01T13:00:00Z" con una duración estimada de "60" minutos y la descripción:
                """
                Reparar una pérdida de agua.
                """
            And esa orden tiene evidencia persistida de sincronización para "ana@example.com" el "2026-09-27T11:00:00Z" y ninguna para "juan@example.com"
            And que la fecha y hora actual del sistema es "2026-09-28T00:00:00Z"
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And el diagnóstico informa Calendar conectado y la orden sin sincronización confirmada para el prestador
            And la evidencia de sincronización del consumidor no se atribuye al prestador
            And la consulta no inicia ni reintenta sincronizaciones

    Rule: La actividad está acotada, usa estados reales y muestra reputación sólo desde reviews persistidas
        Background:
            Given que estoy autenticado como administrador "soporte@example.com" con el permiso "read:providers"

        @wip
        Scenario: 61.1.9-PD Mostrar las seis referencias recientes con identidad y fecha de cada tipo
            Given que la fecha y hora actual del sistema es "2026-10-02T00:00:00Z"
            And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan@example.com"
            And que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor          | prestador        | creada               | estado   |
                | S1         | ana@example.com     | juan@example.com | 2026-09-20T12:00:00Z | pending  |
                | S2         | beatriz@example.com | juan@example.com | 2026-09-21T11:00:00Z | accepted |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada               | fecha programada     | duración | descripción                    | estado   | precio total | moneda | seña    | comisión total | comisión inicial | saldo servicio | saldo comisión |
                | P1        | S2        | 2026-09-21T12:00:00Z | 2026-09-30T13:00:00Z | 60       | Revisar válvula.               | accepted | 8500000      | ARS    | 1700000 | 500000         | 100000           | 6800000        | 400000         |
                | P2        | S2        | 2026-09-21T14:00:00Z | 2026-10-01T13:00:00Z | 60       | Cambiar flexible.              | accepted | 8500000      | ARS    | 1700000 | 500000         | 100000           | 6800000        | 400000         |
                | P3        | S2        | 2026-09-21T14:30:00Z | 2026-10-02T13:00:00Z | 60       | Revisar calefón nuevamente.    | pending  | 8500000      | ARS    | 1700000 | 500000         | 100000           | 6800000        | 400000         |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada             | estado           | finalización informada | saldo pagado          |
                | O1    | P1        | 2026-09-21T15:00:00Z | paid             | 2026-09-30T15:00:00Z   | 2026-10-01T13:00:00Z |
                | O2    | P2        | 2026-09-21T16:00:00Z | awaiting_payment | 2026-10-01T15:00:00Z   |                       |
            And que la orden "O1" tiene una review persistida con calificación 5 y descripción "Trabajo prolijo."
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then la actividad contiene como máximo 6 referencias en total y, para estos datos, en orden más reciente a más antigua:
                | tipo          | referencia | estado           | instante de ordenamiento |
                | work_order    | O2         | awaiting_payment | 2026-09-21T16:00:00Z     |
                | work_order    | O1         | paid             | 2026-09-21T15:00:00Z     |
                | service_proposal | P3      | pending          | 2026-09-21T14:30:00Z     |
                | service_proposal | P2      | accepted         | 2026-09-21T14:00:00Z     |
                | service_proposal | P1      | accepted         | 2026-09-21T12:00:00Z     |
                | job_request   | S2         | accepted         | 2026-09-21T11:00:00Z     |
            And cada fila conserva una referencia tipada e ID estable distinto de sus entidades relacionadas; la solicitud antigua "S1" queda fuera del límite
            And la respuesta informa una review, cantidad 1 y promedio 5, calculados sólo desde la review persistida de la orden pagada "O1"
            And la orden "O2" awaiting_payment no aparece como trabajo pagado ni contribuye al promedio de reviews
            And las referencias no incluyen detalle de operaciones ni chat, y la respuesta no contiene transacciones financieras completas o datos personales del consumidor

        @wip
        Scenario: 61.1.10-PD Informar actividad y reviews vacías como colecciones válidas
            Given que "juan@example.com" no tiene actividad ni reviews asociadas
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And actividad y reviews se informan como colecciones vacías no nulas
            And reputación informa cantidad 0 y promedio 0

    Rule: Autenticación, permiso y auditoría limitan la consulta sin modificar otras autorizaciones
        Scenario Outline: 61.1.11-PD Rechazar el acceso sin token válido
            Given que <autenticación>
            When intento consultar el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 401
            And no se entrega ningún dato del diagnóstico
            And no se registra ningún evento de auditoría para esta solicitud

            Examples:
                | autenticación                 |
                | no envío un token Bearer       |
                | envío un token Bearer inválido |

        Scenario Outline: 61.1.12-PD Denegar la consulta sin el permiso "read:providers"
            Given que <sesión>
            When intento consultar el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 403
            And no se entrega ningún dato del diagnóstico
            And no se registra ningún evento de auditoría para esta solicitud

            Examples:
                | sesión                                                                                       |
                | estoy autenticado como administrador "soporte@example.com" solamente con "read:admin_operations" |
                | estoy autenticado como administrador "soporte@example.com" solamente con "read:admin_audit"      |
                | estoy autenticado como prestador "juan@example.com"                                            |

        Scenario: 61.1.13-PD Informar no encontrado cuando el prestador no existe
            Given que no existe ningún prestador con ID persistido 987654321
            And que estoy autenticado como administrador "soporte@example.com" con el permiso "read:providers"
            When consulto el diagnóstico administrativo del prestador "987654321"
            Then el sistema responde con estado 404
            And no se entrega un diagnóstico
            And no se registra ningún evento de auditoría para esta solicitud

        Scenario: 61.1.14-PD No entregar el diagnóstico cuando falla el registro de auditoría
            Given que estoy autenticado como administrador "soporte@example.com" con el permiso "read:providers"
            And que falla el almacenamiento del evento de auditoría de este diagnóstico
            When consulto el diagnóstico administrativo del prestador "juan@example.com" con correlación "provider-diagnostic-audit-fail"
            Then el sistema responde con estado 500
            And no se entrega ningún campo ni colección del diagnóstico
            And no se persiste ningún evento de auditoría para esta consulta diagnóstica
