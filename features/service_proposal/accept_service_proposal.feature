Feature: Confirmar propuesta de servicio
    Como consumidor
    quiero confirmar el acuerdo del servicio propuesto por el prestador
    para dejar asentada mi aceptación de los términos antes de avanzar con la contratación

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"

    Rule: El consumidor destinatario puede confirmar una propuesta pendiente

        Scenario: 21.1-CSP Confirmar una propuesta de servicio pendiente
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como consumidor "ana@example.com"
            When confirmo la propuesta de servicio pendiente
            Then la propuesta de servicio queda aceptada
            And el sistema registra una única orden de trabajo programada
            And la orden de trabajo queda vinculada a la propuesta aceptada
            And la orden de trabajo conserva el consumidor, el prestador, el monto, la fecha y hora y la descripción acordados

        Scenario: 21.2-CSP Notificar al prestador la confirmación de su propuesta
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
            And que estoy autenticado como consumidor "ana@example.com"
            When confirmo la propuesta de servicio pendiente
            Then el prestador "juan.plomero@example.com" recibe en tiempo real la notificación de propuesta de servicio aceptada

    Rule: Solo el consumidor destinatario puede confirmar la propuesta

        @wip
        Scenario: 21.3-CSP Rechazar confirmación por otro consumidor
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como consumidor "carla@example.com"
            When intento confirmar la propuesta de servicio pendiente de "ana@example.com"
            Then el sistema deniega la confirmación de la propuesta de servicio
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

        @wip
        Scenario: 21.4-CSP Rechazar confirmación por el prestador
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When intento confirmar la propuesta de servicio pendiente
            Then el sistema deniega la confirmación de la propuesta de servicio
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

    Rule: Solamente las propuestas pendientes y vigentes pueden confirmarse

        @wip
        Scenario: 21.5-CSP Rechazar una segunda confirmación de la misma propuesta
            Given que existe una propuesta de servicio aceptada de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento confirmar nuevamente la propuesta de servicio aceptada
            Then el sistema rechaza confirmar una propuesta de servicio ya aceptada
            And el sistema conserva una única orden de trabajo para la propuesta

        @wip
        Scenario: 21.6-CSP Rechazar confirmación de una propuesta rechazada
            Given que existe una propuesta de servicio rechazada de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento confirmar la propuesta de servicio rechazada
            Then el sistema rechaza confirmar una propuesta de servicio rechazada
            And el sistema no registra una orden de trabajo para la propuesta

        @wip
        Scenario: 21.7-CSP Rechazar confirmación de una propuesta vencida
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que la fecha y hora actual del sistema es "2026-07-05T09:30:00-03:00"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento confirmar la propuesta de servicio pendiente
            Then el sistema rechaza la confirmación porque la propuesta de servicio está vencida
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta

    Rule: Confirmar una propuesta no modifica otras propuestas pendientes

        @wip
        Scenario: 21.8-CSP Conservar otras propuestas pendientes entre los participantes
            Given que existen dos propuestas de servicio pendientes de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When confirmo una de las propuestas de servicio pendientes
            Then la propuesta de servicio confirmada queda aceptada
            And la otra propuesta de servicio permanece pendiente
            And el sistema registra una única orden de trabajo para la propuesta aceptada

    Rule: Solo usuarios autenticados pueden confirmar propuestas

        @wip
        Scenario: 21.9-CSP Rechazar confirmación sin sesión válida
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que no tengo una sesión válida
            When intento confirmar la propuesta de servicio pendiente
            Then el sistema deniega el acceso
            And la propuesta de servicio permanece pendiente
            And el sistema no registra una orden de trabajo para la propuesta
