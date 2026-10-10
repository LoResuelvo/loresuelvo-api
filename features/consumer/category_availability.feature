Feature: Descubrir prestadores y continuar solicitudes según la disponibilidad del rubro
    Como consumidor
    quiero encontrar ofertas en rubros habilitados y continuar mis solicitudes existentes
    para contratar servicios sin que un rubro deshabilitado genere nuevas operaciones

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"

    Rule: Un rubro deshabilitado no ofrece prestadores para nuevas solicitudes

        Scenario: 38.2.20-BR No ofrecer prestadores al buscar por el identificador de un rubro deshabilitado
            Given que el rubro "Plomería" se deshabilitó después del registro de "juan.plomero@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When filtro técnicos usando el identificador guardado del rubro "Plomería"
            Then el sistema muestra un listado de técnicos vacío

        Scenario: 38.2.21-EST Rechazar una solicitud manual a un prestador de un rubro deshabilitado
            Given que el rubro "Plomería" se deshabilitó después del registro de "juan.plomero@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento enviar una solicitud de trabajo al prestador "Juan Gómez" con el título "Reparación de una pérdida" y la descripción:
                """
                Necesito reparar una pérdida de agua debajo de la pileta.
                """
            Then el sistema rechaza la solicitud porque el rubro del prestador no está habilitado para nuevas operaciones
            And el sistema no registra una solicitud de trabajo para "Juan Gómez"

        Scenario: 38.2.22-SAI Rechazar una solicitud desde una recomendación guardada antes de deshabilitar el rubro
            Given que estoy autenticado como consumidor "ana@example.com"
            And que el chatbot asistido por IA está disponible
            And que tengo una conversación con el chatbot cuya evaluación vigente requiere un profesional del rubro "Plomería" con el título "Pérdida debajo de la pileta" y la descripción:
                """
                Hay una pérdida de agua persistente debajo de la pileta.
                """
            And que la recomendación de "Juan Gómez" quedó guardada mientras el rubro "Plomería" estaba habilitado
            And que el rubro "Plomería" se deshabilitó después de guardar esa recomendación
            When intento contactar al prestador recomendado "Juan Gómez" desde esa conversación con el chatbot
            Then el sistema rechaza la solicitud porque el rubro evaluado no está habilitado para nuevas operaciones
            And el sistema no registra una solicitud de trabajo para "Juan Gómez"

    Rule: Deshabilitar un rubro no elimina las relaciones existentes

        Scenario: 38.2.23-VPP Mostrar el rubro del prestador en una orden existente aunque esté deshabilitado
            Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
            And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que el rubro "Plomería" se deshabilitó después del registro de "juan.plomero@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis órdenes de trabajo
            Then el sistema muestra la orden de trabajo programada por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00"
            And la contraparte de la orden de trabajo es el prestador "Juan Gómez" con rubro "Plomería" y su foto de perfil

    Rule: Reactivar un rubro restituye la oferta para nuevas operaciones

        Scenario: 38.2.24-BR Volver a ofrecer prestadores después de reactivar el mismo rubro
            Given que el rubro "Plomería" se deshabilitó y luego se reactivó con el mismo identificador
            And que estoy autenticado como consumidor "ana@example.com"
            When filtro técnicos usando el identificador guardado del rubro "Plomería"
            Then el sistema muestra al técnico "Juan Gómez"
