@wip
Feature: Buscar técnicos por rubro
    Como consumidor
    quiero buscar técnicos por rubro
    para encontrar profesionales especializados en el servicio que necesito

    Background:
        Given que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        Given que existe el rubro "Gasista"
        And existe un prestador registrado con correo "laura.electricista@example.com", nombre "Laura", apellido "Gómez" y rubro "Electricidad"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Pérez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    Rule: La búsqueda debe mostrar solo técnicos del rubro seleccionado

    Scenario: 01-BR Buscar técnicos por rubro correctamente
        When busco técnicos del rubro "Plomería"
        Then el sistema muestra al técnico "Juan Pérez"
        And el sistema muestra al técnico "Pedro Dib"

    Scenario: 02-BR Buscar técnicos por rubro ignorando mayúsculas y espacios
        When busco técnicos del rubro "  electricidad  "
        Then el sistema muestra solamente al técnico "Laura Gómez" en el resultado

    Rule: La búsqueda debe informar cuando no hay técnicos disponibles para el rubro

    Scenario: 03-BR Mostrar listado vacío cuando no hay técnicos del rubro seleccionado
        When busco técnicos del rubro "Gasista"
        Then el sistema muestra un listado de técnicos vacío

    Rule: El consumidor debe indicar un rubro válido para buscar

    Scenario: 04-BR Rechazar búsqueda sin rubro
        When intento buscar técnicos sin indicar rubro
        Then el sistema me indica que el rubro es obligatorio

    Scenario: 05-BR Rechazar búsqueda por rubro inexistente
        When busco técnicos del rubro "Carpintería"
        Then el sistema me indica que el rubro no existe
