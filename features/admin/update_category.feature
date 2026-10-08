Feature: Editar, desactivar y reactivar rubros
    Como administrador de LoResuelvo
    quiero mantener los rubros del catálogo
    para actualizar la oferta sin interrumpir las operaciones existentes

    Background:
        Given que existe un administrador provisionado con correo "supervisor@example.com", nombre "Sofía" y apellido "López"

    Rule: Un administrador autorizado puede editar un rubro existente

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "write:categories"

        @wip
        Scenario: 38.2.1-CR Renombrar un rubro conservando su identidad
            Given que existe el rubro "Plomería"
            When cambio el nombre del rubro "Plomería" por "Instalaciones sanitarias" con la versión actual y el motivo "Actualizar el nombre visible"
            Then el sistema actualiza el rubro existente
            And la respuesta contiene el identificador original, el nombre "Instalaciones sanitarias", su estado habilitado y la nueva versión
            And queda registrado un único evento de edición de este rubro por "supervisor@example.com" con el motivo "Actualizar el nombre visible"

        @wip
        Scenario: 38.2.2-CR Permitir conservar el nombre normalizado propio
            Given que existe el rubro "Plomería"
            When cambio el nombre del rubro "Plomería" por "  PLOMERÍA  " con la versión actual
            Then el sistema actualiza el nombre visible a "PLOMERÍA"
            And el rubro conserva su identificador y permanece habilitado

        @wip
        Scenario: 38.2.3-CR Rechazar el nombre normalizado de otro rubro deshabilitado
            Given que existe el rubro habilitado "Plomería"
            And que existe el rubro deshabilitado "Electricidad"
            When intento cambiar el nombre del rubro "Plomería" por "  ELECTRICIDAD  " con la versión actual
            Then el sistema rechaza el cambio porque el nombre del rubro ya existe
            And ambos rubros conservan sus nombres y estados
            And no queda registrado un evento exitoso de edición de esta solicitud

        @wip
        Scenario: 38.2.4-CR Requerir la versión esperada para editar un rubro
            Given que existe el rubro "Plomería"
            When intento cambiar el nombre del rubro "Plomería" sin indicar la versión esperada
            Then el sistema rechaza la edición con estado 400
            And el rubro conserva su nombre, estado y versión

        @wip
        Scenario: 38.2.5-CR Rechazar una edición con una versión anterior
            Given que existe el rubro "Plomería" en la versión 1
            And que el mismo administrador mantiene abierta otra pestaña con la versión 1 del rubro
            And que desde la primera pestaña ya cambió el nombre del rubro a "Instalaciones sanitarias"
            When intento cambiar desde la otra pestaña el nombre del rubro "Plomería" por "Servicios sanitarios" usando la versión 1
            Then el sistema rechaza la edición con estado 409 porque la versión del rubro cambió
            And el rubro conserva el nombre "Instalaciones sanitarias" y sólo registra el cambio efectivo

    Rule: Deshabilitar y reactivar requiere motivo y conserva las operaciones existentes

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "write:categories"

        @wip
        Scenario: 38.2.6-CR Rechazar un cambio de estado sin motivo
            Given que existe el rubro habilitado "Plomería"
            When intento deshabilitar el rubro "Plomería" con la versión actual y sin motivo
            Then el sistema rechaza el cambio con estado 400
            And el rubro permanece habilitado con la misma versión
            And no queda registrado un evento exitoso de edición de esta solicitud

        @wip
        Scenario: 38.2.7-CR Deshabilitar un rubro sin órdenes en curso
            Given que existe el rubro habilitado "Plomería" sin órdenes en curso
            When deshabilito el rubro "Plomería" y cambio su nombre a "Instalaciones sanitarias" con la versión actual y el motivo "Retirar temporalmente del catálogo"
            Then el sistema devuelve el mismo rubro deshabilitado con el nombre "Instalaciones sanitarias" y su nueva versión
            And queda registrado un único evento para esta solicitud, correspondiente a la desactivación de este rubro por "supervisor@example.com" con el motivo "Retirar temporalmente del catálogo" y los estados anterior habilitado y posterior deshabilitado

        @wip
        Scenario: 38.2.8-CR Reactivar el rubro deshabilitado con su misma identidad
            Given que existe el rubro deshabilitado "Plomería"
            When habilito el rubro "Plomería" con la versión actual y el motivo "Retomar la oferta del servicio"
            Then el sistema devuelve el mismo rubro habilitado con su nueva versión
            And queda registrado un único evento de reactivación de este rubro por "supervisor@example.com" con el motivo "Retomar la oferta del servicio"

        @wip
        Scenario: 38.2.9-CR Exigir confirmación adicional si hay una orden programada
            Given que existe el rubro habilitado "Plomería"
            And que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
            And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que existe la solicitud de trabajo aceptada "S1" de "ana@example.com" para "juan.plomero@example.com"
            And que la conversación de trabajo vinculada a la solicitud "S1" está activa
            And que existe una orden de trabajo programada para la propuesta aceptada en la conversación de la solicitud "S1" de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-17T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            When intento deshabilitar el rubro "Plomería" con la versión actual y el motivo "Retirar temporalmente del catálogo" sin confirmar las órdenes en curso
            Then el sistema rechaza el cambio con estado 409 porque se requiere confirmar las órdenes en curso
            And el rubro permanece habilitado
            And la orden de trabajo conserva su estado programado
            And no queda registrado un evento exitoso de edición de esta solicitud

        @wip
        Scenario: 38.2.10-CR Deshabilitar con confirmación sin cancelar la orden existente
            Given que existe el rubro habilitado "Plomería"
            And que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
            And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que existe la solicitud de trabajo aceptada "S1" de "ana@example.com" para "juan.plomero@example.com"
            And que la conversación de trabajo vinculada a la solicitud "S1" está activa
            And que existe una orden de trabajo programada para la propuesta aceptada en la conversación de la solicitud "S1" de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-17T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            When deshabilito el rubro "Plomería" con la versión actual, el motivo "Retirar temporalmente del catálogo" y confirmo las órdenes en curso
            Then el sistema devuelve el mismo rubro deshabilitado con su nueva versión
            And la orden de trabajo conserva su estado programado
            And queda registrado un único evento para esta solicitud, correspondiente a la desactivación de este rubro por "supervisor@example.com" con el motivo "Retirar temporalmente del catálogo" y los estados anterior habilitado y posterior deshabilitado

        @wip
        Scenario: 38.2.11-CR Aceptar una edición que conserva los valores actuales sin duplicar el cambio
            Given que existe el rubro habilitado "Plomería" en la versión 1
            When actualizo el rubro "Plomería" con su mismo nombre y estado habilitado usando la versión 1
            Then el sistema responde con estado 200 y devuelve el rubro con la versión 1
            And no queda registrado un evento de edición para esta solicitud

    Rule: Solo un administrador con el permiso requerido puede editar rubros

        @wip
        Scenario: 38.2.12-CR Rechazar la edición sin el permiso requerido
            Given que existe el rubro "Plomería"
            And que estoy autenticado como administrador "supervisor@example.com" solamente con el permiso "read:categories"
            When intento cambiar el nombre del rubro "Plomería" por "Instalaciones sanitarias" con la versión actual
            Then el sistema responde con estado 403
            And el rubro conserva su nombre, estado y versión
            And no queda registrado un evento exitoso de edición de esta solicitud
