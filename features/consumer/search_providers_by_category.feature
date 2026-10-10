Feature: Buscar prestadores por rubro y zona del domicilio del consumidor
    Como consumidor
    quiero buscar prestadores de un rubro que cubran la zona de mi domicilio
    para encontrar profesionales que puedan atenderme

    Background:
        Given que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        And que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que el domicilio del consumidor "ana@example.com" pertenece a la zona de cobertura "Comuna 6"
        And que estoy autenticado como consumidor "ana@example.com"

    Rule: La búsqueda muestra únicamente prestadores del rubro que cubren la zona del domicilio del consumidor

        @wip
        Scenario: 01-BR Filtrar por rubro y cobertura de la zona del domicilio
            Given existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y zona de cobertura "Comuna 6"
            And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 14"
            And existe un prestador registrado con correo "laura.electricista@example.com", nombre "Laura", apellido "Suárez", rubro "Electricidad" y zona de cobertura "Comuna 6"
            When filtro técnicos por el rubro "Plomería"
            Then el sistema muestra solamente al técnico "Juan Gómez" en el resultado

        @wip
        Scenario: 02-BR Filtrar técnicos por rubro con múltiples técnicos registrados
            Given existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y zona de cobertura "Comuna 6"
            And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 6"
            When filtro técnicos por el rubro "Plomería"
            Then el sistema muestra al técnico "Juan Gómez"
            And el sistema muestra al técnico "Pedro Dib"

        @wip
        Scenario: 03-BR Mostrar listado vacío cuando ningún técnico del rubro cubre la zona del domicilio
            Given existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 14"
            When filtro técnicos por el rubro "Plomería"
            Then el sistema muestra un listado de técnicos vacío

    Rule: El consumidor debe indicar un rubro válido para filtrar

        @wip
        Scenario: 04-BR Rechazar filtro sin rubro
            When intento filtrar técnicos sin indicar rubro
            Then el sistema me indica que el rubro es obligatorio

        @wip
        Scenario: 05-BR Rechazar filtro por rubro inexistente
            When filtro técnicos por un rubro inexistente
            Then el sistema me indica que el rubro no existe

    Rule: Un prestador puede cubrir varias zonas, incluida la del domicilio del consumidor

        @wip
        Scenario: 8.1.1-BR Incluir un prestador que cubre varias zonas, incluida la del domicilio
            Given que el domicilio del consumidor "ana@example.com" pertenece a la zona de cobertura "Comuna 14"
            And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y las zonas de cobertura "Comuna 6" y "Comuna 14"
            And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 6"
            When filtro técnicos por el rubro "Plomería"
            Then el sistema muestra solamente al técnico "Juan Gómez" en el resultado

    Rule: La búsqueda requiere autenticación

        @wip
        Scenario: 8.1.2-BR Rechazar la búsqueda sin sesión válida
            Given que no tengo una sesión válida
            When filtro técnicos por el rubro "Plomería"
            Then el sistema deniega el acceso
