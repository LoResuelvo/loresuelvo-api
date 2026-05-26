Feature: Buscar técnicos por rubro
    Como consumidor
    quiero buscar técnicos por rubro
    para encontrar profesionales especializados en el servicio que necesito

    Background:
        Given que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        And que existe el rubro "Gasista"
        And existe un prestador registrado con correo "laura.electricista@example.com", nombre "Laura", apellido "Gómez" y rubro "Electricidad"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Pérez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    Rule: El filtro debe mostrar solo técnicos del rubro seleccionado

    Scenario: 01-BR Filtrar técnicos por rubro con un solo técnico registrado
        When filtro técnicos por el rubro "Electricidad"
        Then el sistema muestra solamente al técnico "Laura Gómez" en el resultado

    Scenario: 02-BR Filtrar técnicos por rubro con múltiples técnicos registrados
        When filtro técnicos por el rubro "Plomería"
        Then el sistema muestra al técnico "Juan Pérez"
        And el sistema muestra al técnico "Pedro Dib"

    Rule: El filtro debe informar cuando no hay técnicos disponibles para el rubro

    Scenario: 03-BR Mostrar listado vacío cuando no hay técnicos del rubro seleccionado
        When filtro técnicos por el rubro "Gasista"
        Then el sistema muestra un listado de técnicos vacío

    Rule: El consumidor debe indicar un rubro válido para filtrar

    Scenario: 04-BR Rechazar filtro sin rubro
        When intento filtrar técnicos sin indicar rubro
        Then el sistema me indica que el rubro es obligatorio

    Scenario: 05-BR Rechazar filtro por rubro inexistente
        When filtro técnicos por un rubro inexistente
        Then el sistema me indica que el rubro no existe
