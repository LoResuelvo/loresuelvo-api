Feature: Consultar el impacto operativo de un rubro
    Como administrador de LoResuelvo
    quiero conocer la actividad asociada a un rubro
    para decidir informadamente si lo deshabilito

    Background:
        Given que existe un administrador provisionado con correo "supervisor@example.com", nombre "Sofía" y apellido "López"

    Rule: Un administrador autorizado puede consultar el impacto sin modificar las operaciones

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:categories"

        @wip
        Scenario: 38.2.13-IR Consultar el impacto de un rubro con actividad
            Given que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
            And que existe el rubro habilitado "Plomería"
            And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que existe la solicitud de trabajo aceptada "S1" de "ana@example.com" para "juan.plomero@example.com"
            And que la conversación de trabajo vinculada a la solicitud "S1" está activa
            And que existe una orden de trabajo programada para la propuesta aceptada en la conversación de la solicitud "S1" de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-17T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            When consulto el impacto del rubro "Plomería"
            Then el impacto informa el rubro habilitado y su versión a la fecha "2026-08-15T14:00:00Z"
            And el impacto cuenta una asignación de prestador y una orden programada
            And el impacto informa cero solicitudes pendientes, aceptadas sin propuesta, propuestas pendientes y órdenes awaiting_payment
            And el impacto indica que hay órdenes en curso y se requiere confirmación adicional para deshabilitar el rubro
            And el impacto informa que las operaciones existentes, incluidas las negociaciones previas, pueden continuar aunque se deshabilite el rubro
            And la orden de trabajo conserva su estado programado y la consulta no modifica las operaciones

        @wip
        Scenario: 38.2.14-IR Consultar un rubro deshabilitado sin actividad
            Given que existe el rubro deshabilitado "Electricidad"
            And que no hay prestadores ni operaciones asociadas al rubro "Electricidad"
            When consulto el impacto del rubro "Electricidad"
            Then el impacto informa el rubro deshabilitado y su versión
            And los seis conteos de impacto son cero
            And la consulta no registra un evento de auditoría ni modifica el rubro

    Rule: El impacto requiere el permiso de lectura de rubros

        @wip
        Scenario: 38.2.15-IR Rechazar la consulta de impacto sin el permiso requerido
            Given que existe el rubro "Plomería"
            And que estoy autenticado como administrador "supervisor@example.com" solamente con el permiso "read:admin_operations"
            When consulto el impacto del rubro "Plomería"
            Then el sistema responde con estado 403

        @wip
        Scenario: 38.2.16-IR Rechazar la consulta de impacto sin autenticación
            Given que existe el rubro "Plomería"
            And que no tengo una sesión válida
            When consulto el impacto del rubro "Plomería"
            Then el sistema deniega el acceso
