Feature: Consultar el detalle administrativo de un reclamo
    Como operador autorizado de soporte y mediación
    quiero consultar el expediente completo y sus evidencias autorizadas
    para revisar lo registrado sin alterar la contratación ni revelar información ajena

    Background:
        Given que existen el consumidor registrado "ana@example.com" y el prestador registrado "juan@example.com"
        And que ambos participan en la solicitud de trabajo "S1"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: El detalle expone el expediente y sólo sus evidencias vinculadas

        @wip
        Scenario: 68.7-GC Entregar el detalle con enlaces temporales de evidencias privadas
            Given que la solicitud "S1" no tiene propuesta, orden ni intento de pago
            And que "juan@example.com" tiene el reclamo "C1" sobre "S1" en estado "open", reportado el "2026-09-24T10:00:00-03:00", con motivo "non_compliance" y testimonio "La reparación pactada no quedó completa"
            And que "C1" conserva la identidad canónica "jr-" seguida del ID persistido de "S1" y la referencia "job_request_id" de "S1"
            And que "C1" tiene la imagen privada confirmada "IMG1" vinculada
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto el detalle administrativo del reclamo "C1"
            Then el sistema responde con estado 200 y el detalle de "C1"
            And el detalle informa el tipo de reclamante "provider"
            And el detalle presenta el motivo "non_compliance", el testimonio original "La reparación pactada no quedó completa" y la fecha de reporte "2026-09-24T10:00:00-03:00"
            And el detalle conserva la identidad canónica "jr-" seguida del ID persistido de "S1" y la referencia específica "job_request_id" de "S1"
            And el detalle incluye la imagen vinculada "IMG1" con su identificador y una URL temporal privada
            And la respuesta no expone claves de almacenamiento ni credenciales
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        @wip
        Scenario: 68.8-GC Incluir resolución y actuaciones sin reproducir transacciones financieras
            Given que "ana@example.com" tiene el reclamo finalizado "C1" con su dictamen formal persistido por el contrato compartido con US-31
            And que el dictamen de "C1" tiene tipo "consumer_favor", fundamentación "Se verificó el incumplimiento acordado" y fecha "2026-09-24T12:00:00Z"
            And que el dictamen sugiere una compensación de 1500 centavos ARS, sin afirmar que se ejecutó
            And que el expediente conserva su historial de actuaciones y estados
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto el detalle administrativo del reclamo "C1"
            Then el sistema responde con estado 200
            And el detalle presenta el tipo, fundamentación, fecha, compensación sugerida e historial persistidos
            And la respuesta no contiene mensajes de chat ni transacciones financieras completas
            And la compensación se identifica como sugerida y no ejecutada

        @wip
        Scenario: 68.9-GC Informar una colección vacía cuando el expediente no tiene evidencias
            Given que "ana@example.com" tiene el reclamo "C1" sin evidencias vinculadas
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto el detalle administrativo del reclamo "C1"
            Then el sistema responde con estado 200
            And el detalle informa las evidencias como una colección vacía, no nula

        @wip
        Scenario: 68.10-GC No incluir evidencias de otro reclamo aunque pertenezcan a la misma operación
            Given que "ana@example.com" tiene el reclamo "C1" y "juan@example.com" tiene el reclamo "C2" sobre la misma operación
            And la imagen privada confirmada "IMG1" está vinculada a "C1" y no a "C2"
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto el detalle administrativo del reclamo "C2"
            Then el sistema responde con estado 200
            And el detalle no incluye "IMG1" ni una URL temporal para ese archivo

    Rule: La lectura administrativa se audita antes de resolver las evidencias

        @wip
        Scenario: 68.11-GC Auditar el acceso antes de entregar detalle y URLs temporales
            Given que "ana@example.com" tiene el reclamo "C1" con la imagen privada confirmada "IMG1" vinculada
            And no existe un evento de auditoría para la correlación "claim-detail-1"
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto el detalle administrativo del reclamo "C1" con la correlación "claim-detail-1"
            Then el sistema responde con estado 200 y el detalle de "C1"
            And antes de generar cualquier URL temporal queda persistido exactamente un evento de acceso al reclamo "C1" por "operador@example.com" y la correlación "claim-detail-1" con resultado "prepared"
            And el expediente y sus actuaciones permanecen sin cambios

        @wip
        Scenario: 68.12-GC No entregar detalle ni URLs cuando falla la auditoría de acceso
            Given que "ana@example.com" tiene el reclamo "C1" con la imagen privada confirmada "IMG1" vinculada
            And falla el registro de auditoría de acceso al reclamo "C1"
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto el detalle administrativo del reclamo "C1"
            Then el sistema responde con estado 500
            And la respuesta no contiene detalle ni evidencias
            And no se genera ni entrega ninguna URL temporal

    Rule: El permiso específico y la identidad del expediente limitan el detalle

        @wip
        Scenario: 68.13-GC Rechazar el detalle sin permiso de lectura de reclamos
            Given que "ana@example.com" tiene el reclamo "C1" con la imagen privada confirmada "IMG1" vinculada
            And estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_audit"
            When consulto el detalle administrativo del reclamo "C1"
            Then el sistema responde con estado 403
            And la respuesta no contiene datos del expediente ni URLs temporales
