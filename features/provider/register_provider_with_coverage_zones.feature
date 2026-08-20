Feature: Registrar prestador con zonas de cobertura
    Como prestador
    quiero seleccionar las comunas en las que estoy dispuesto a trabajar
    para recibir solicitudes únicamente dentro de mi cobertura

    Background:
        Given que existe el rubro "Plomería"
        And que no existe un usuario con correo "prestador@example.com"
        And que cargué una foto de perfil válida
        And que están habilitadas las zonas de cobertura "Comuna 6", "Comuna 14" y "Comuna 15"

    Rule: El prestador debe seleccionar al menos una zona de cobertura habilitada

        @wip
        Scenario: 35.5.1-RPWCZ Registrar un prestador con una zona de cobertura
            When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería" y zona de cobertura "Comuna 6"
            Then el sistema confirma el registro
            And el prestador "prestador@example.com" queda registrado con la zona de cobertura "Comuna 6"

        @wip
        Scenario: 35.5.2-RPWCZ Rechazar el registro sin zonas de cobertura
            When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería" y sin zonas de cobertura
            Then el sistema me indica que debo seleccionar al menos una zona de cobertura
            And el prestador "prestador@example.com" no queda registrado

        @wip
        Scenario: 35.5.3-RPWCZ Rechazar el registro con una zona de cobertura inexistente
            Given que no existe la zona de cobertura "Comuna 99"
            When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería" y zona de cobertura "Comuna 99"
            Then el sistema me indica que la zona de cobertura seleccionada no está disponible
            And el prestador "prestador@example.com" no queda registrado

        @wip
        Scenario: 35.5.4-RPWCZ Rechazar el registro con una zona de cobertura deshabilitada
            Given que la zona de cobertura "Comuna 15" está deshabilitada
            When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería" y zona de cobertura "Comuna 15"
            Then el sistema me indica que la zona de cobertura seleccionada no está disponible
            And el prestador "prestador@example.com" no queda registrado

        @wip
        Scenario: 35.5.5-RPWCZ Rechazar una zona de cobertura seleccionada más de una vez
            When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería" y selecciono dos veces la zona de cobertura "Comuna 6"
            Then el sistema me indica que una zona de cobertura no puede seleccionarse más de una vez
            And el prestador "prestador@example.com" no queda registrado
