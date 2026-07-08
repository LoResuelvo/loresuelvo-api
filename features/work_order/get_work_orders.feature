@wip
Feature: 21.1 Obtener órdenes de trabajo
    Como usuario
    quiero obtener mis órdenes de trabajo
    para poder consultar los servicios contratados y darles seguimiento

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.electricista@example.com", nombre "Pedro", apellido "Dib" y rubro "Electricidad"

    Rule: Un usuario puede listar las órdenes de trabajo en las que participa

        Scenario: 21.1.1-GWO Consumidor sin órdenes de trabajo obtiene un listado vacío
            Given que estoy autenticado como consumidor "ana@example.com"
            When consulto mis órdenes de trabajo
            Then el sistema muestra un listado de órdenes de trabajo vacío

        Scenario: 21.1.2-GWO Consumidor obtiene una orden de trabajo contratada
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis órdenes de trabajo
            Then el sistema muestra la orden de trabajo programada por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00"
            And la orden de trabajo incluye la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And la orden de trabajo incluye el identificador de la propuesta de servicio aceptada
            And la orden de trabajo incluye la fecha y hora de aceptación de la propuesta
            And la contraparte de la orden de trabajo es el prestador "Juan Gómez" con rubro "Plomería" y su foto de perfil

        Scenario: 21.1.3-GWO Prestador obtiene una orden de trabajo contratada
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When consulto mis órdenes de trabajo
            Then el sistema muestra la orden de trabajo programada por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00"
            And la orden de trabajo incluye la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And la orden de trabajo incluye el identificador de la propuesta de servicio aceptada
            And la contraparte de la orden de trabajo es la consumidora "Ana Pérez"
            And la contraparte no incluye un rubro

    Rule: El listado solo incluye órdenes propias y muestra primero las más próximas

        Scenario: 21.1.4-GWO Usuario obtiene solamente las órdenes de trabajo en las que participa
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com"
            And que existe una orden de trabajo programada para la propuesta aceptada de "pedro.electricista@example.com" para "carla@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis órdenes de trabajo
            Then el sistema muestra solamente la orden de trabajo entre "ana@example.com" y "juan.plomero@example.com"

        Scenario: 21.1.5-GWO Usuario obtiene primero las órdenes de trabajo más próximas
            Given que existen varias órdenes de trabajo programadas para "ana@example.com" con distintas fechas y horas de servicio
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis órdenes de trabajo
            Then el sistema muestra las órdenes de trabajo ordenadas desde la fecha y hora de servicio más próxima

    Rule: Solo usuarios autenticados pueden listar órdenes de trabajo

        Scenario: 21.1.6-GWO Rechazar listado sin sesión válida
            Given que no tengo una sesión válida
            When intento consultar mis órdenes de trabajo
            Then el sistema deniega el acceso
