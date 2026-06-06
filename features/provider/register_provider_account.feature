Feature: Registrar cuenta nueva de prestador
    Como prestador
    quiero registrarme en la plataforma
    para poder ofrecer mis servicios a los consumidores

    Background:
        Given que existe el rubro "Plomería"
        # And que existe el rubro "Gasista matriculado"
        And que no existe un usuario con correo "prestador@example.com"

    Scenario: 01-RPA Registrar una cuenta nueva de prestador correctamente
        And que cargué una foto de perfil válida
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez" y rubro "Plomería"
        Then el sistema confirma el registro

    Rule: El prestador debe indicar su rubro

    Scenario: 02-RPA Rechazar registro sin rubro
        And que cargué una foto de perfil válida
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez" y sin rubro
        Then el sistema me indica que el rubro es obligatorio
    
    Rule: El prestador debe indicar su zona de cobertura
    
    @wip
    Scenario: 03-RPA Rechazar registro sin zona de cobertura
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería" y sin zona de cobertura
        Then el sistema me indica que la zona de cobertura es obligatoria

    Rule: El prestador debe presentar documentación obligatoria
    @wip
    Scenario: 04-RPA Rechazar registro sin antecedentes penales
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería", zona de cobertura "Zona Norte" sin presentar antecedentes penales
        Then el sistema me indica que los antecedentes penales son obligatorios
    @wip
    Scenario: 05-RPA Rechazar registro sin constancia de CUIT
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería", zona de cobertura "Zona Norte" sin presentar constancia de CUIT
        Then el sistema me indica que la constancia de CUIT es obligatoria
    @wip
    Scenario: 06-RPA Rechazar registro sin validación biométrica
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería", zona de cobertura "Zona Norte" sin completar la validación biométrica
        Then el sistema me indica que la validación biométrica es obligatoria

    Rule: Algunos rubros requieren matrícula, certificación o habilitación profesional
    @wip
    Scenario: 07-RPA Registrar prestador en rubro que no requiere matrícula profesional
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Plomería", zona de cobertura "Zona Norte" e ingreso mis documentos obligatorios sin matrícula profesional
        Then el sistema confirma el registro
    @wip
    Scenario: 08-RPA Rechazar registro en rubro que requiere matrícula profesional
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez", rubro "Gasista matriculado", zona de cobertura "Zona Norte" e ingreso mis documentos obligatorios sin matrícula profesional
        Then el sistema me indica que la matrícula, certificación o habilitación profesional es obligatoria para el rubro
