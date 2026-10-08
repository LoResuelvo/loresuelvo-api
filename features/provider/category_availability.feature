Feature: Operar con rubros deshabilitados
    Como prestador
    quiero conservar las operaciones aceptadas antes de que se deshabilite mi rubro
    para continuar trabajando sin aceptar nuevas solicitudes en ese rubro

    Rule: Un rubro deshabilitado no admite nuevas inscripciones

        Scenario: 38.2.25-RPA Rechazar el registro de un prestador en un rubro deshabilitado
            Given que existe el rubro deshabilitado "Plomería"
            And que no existe un usuario con correo "nuevo.prestador@example.com"
            And que tengo una sesión válida para una identidad que aún no está registrada
            And que están habilitadas las zonas de cobertura "Comuna 6"
            And que cargué una foto de perfil válida
            When me registro como prestador con correo "nuevo.prestador@example.com", nombre "Luis", apellido "García", rubro "Plomería" y zona de cobertura "Comuna 6"
            Then el sistema rechaza el registro porque el rubro no está habilitado para nuevas operaciones
            And el sistema no registra una cuenta de prestador para "nuevo.prestador@example.com"

    Rule: Un rubro deshabilitado no interrumpe una negociación iniciada antes de su desactivación

        Scenario: 38.2.26-PSP Crear una propuesta para una solicitud aceptada antes de deshabilitar el rubro
            Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
            And que existe el rubro "Plomería"
            And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"
            And que existe una solicitud de trabajo aceptada entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com"
            And que el rubro "Plomería" se deshabilitó después de aceptar esa solicitud
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When envío una propuesta de servicio al consumidor "ana@example.com" por "15000.50" para la fecha y hora "2026-07-06T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            Then el sistema registra la propuesta de servicio para la solicitud existente
            And la solicitud de trabajo permanece aceptada
