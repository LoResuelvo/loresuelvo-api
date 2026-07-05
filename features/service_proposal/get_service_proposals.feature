Feature: Obtener propuestas de servicio
    Como usuario
    quiero obtener mis propuestas de trabajo
    para poder consultarlas y darles seguimiento

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    Rule: Un usuario puede listar las propuestas de servicio en las que participa

        Scenario: 54.1-GSP Consumidor sin propuestas obtiene un listado vacío
            Given que estoy autenticado como consumidor "ana@example.com"
            When consulto mis propuestas de servicio
            Then el sistema muestra un listado de propuestas de servicio vacío
        
        Scenario: 54.2-GSP Consumidor obtiene una propuesta recibida
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis propuestas de servicio
            Then el sistema muestra la propuesta de servicio pendiente por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00"
            And la propuesta incluye la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And la contraparte de la propuesta es el prestador "Juan Gómez" con rubro "Plomería" y su foto de perfil
            And la propuesta incluye el identificador de la conversación con el prestador
        
        Scenario: 54.3-GSP Prestador obtiene una propuesta enviada
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When consulto mis propuestas de servicio
            Then el sistema muestra la propuesta de servicio pendiente por "15000.50" para la fecha y hora "2026-07-05T09:30:00-03:00"
            And la propuesta incluye la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And la contraparte de la propuesta es la consumidora "Ana Pérez"
            And la contraparte no incluye un rubro
            And la propuesta incluye el identificador de la conversación con la consumidora

    Rule: El listado permite distinguir el estado de cada propuesta
        
        Scenario: 54.4-GSP Usuario obtiene propuestas con distintos estados
            Given que existen propuestas de servicio de "juan.plomero@example.com" para "ana@example.com" con los estados:
                | estado   |
                | pending  |
                | accepted |
                | rejected |
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis propuestas de servicio
            Then el sistema muestra las 3 propuestas de servicio
            And cada propuesta incluye su estado actual

    Rule: El listado solo incluye propuestas propias y muestra primero las más recientes
        
        Scenario: 54.5-GSP Usuario obtiene solamente las propuestas en las que participa
            Given que existe una propuesta de servicio pendiente de "juan.plomero@example.com" para "ana@example.com"
            And que existe una propuesta de servicio pendiente de "pedro.plomero@example.com" para "carla@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis propuestas de servicio
            Then el sistema muestra solamente la propuesta entre "ana@example.com" y "juan.plomero@example.com"
        
        Scenario: 54.6-GSP Usuario obtiene primero las propuestas más recientes
            Given que existen varias propuestas de servicio para "ana@example.com" creadas en distintos momentos
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis propuestas de servicio
            Then el sistema muestra las propuestas de servicio ordenadas desde la más reciente

    Rule: Solo usuarios autenticados pueden listar propuestas de servicio
        
        Scenario: 54.7-GSP Rechazar listado sin sesión válida
            Given que no tengo una sesión válida
            When intento consultar mis propuestas de servicio
            Then el sistema deniega el acceso
