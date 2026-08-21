Feature: Listar zonas de cobertura disponibles
    Como prestador
    quiero consultar las zonas de cobertura habilitadas
    para seleccionar aquellas en las que estoy dispuesto a trabajar durante mi registro

    Background:
        Given que el market de cobertura "CABA" está habilitado

    Rule: El catálogo debe informar las zonas habilitadas de CABA disponibles para el registro

        @wip
        Scenario: 35.5.1.1-LACZ Listar las zonas de cobertura habilitadas
            Given que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14" en el market "CABA"
            When consulto el listado de zonas de cobertura disponibles
            Then el sistema responde exitosamente con las siguientes zonas de cobertura:
                | nombre    |
                | Comuna 6  |
                | Comuna 14 |
            And cada zona de cobertura incluye un identificador interno estable

        @wip
        Scenario: 35.5.1.2-LACZ Excluir las zonas de cobertura deshabilitadas
            Given que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14" en el market "CABA"
            And que la zona de cobertura "Comuna 14" está deshabilitada
            When consulto el listado de zonas de cobertura disponibles
            Then el listado incluye la zona de cobertura "Comuna 6"
            And el listado no incluye la zona de cobertura "Comuna 14"

        @wip
        Scenario: 35.5.1.5-LACZ Devolver un listado vacío cuando no hay zonas habilitadas
            Given que no existen zonas de cobertura habilitadas en el market "CABA"
            When consulto el listado de zonas de cobertura disponibles
            Then el sistema responde exitosamente con un listado de zonas de cobertura vacío
