@wip
Feature: Completar registro de prestador con foto de perfil
    Como prestador
    quiero cargar una foto de perfil al registrarme
    para que los consumidores puedan reconocerme en la plataforma

    Background:
        Given que existe el rubro "Plomería"
        And que no existe un usuario con correo "prestador@example.com"

    Scenario: 01-RPF Registrar una cuenta nueva de prestador con foto de perfil correctamente
        Given que cargué una foto de perfil válida
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez" y rubro "Plomería"
        Then el sistema confirma el registro

    Rule: El prestador debe cargar una foto de perfil

    Scenario: 02-RPF Rechazar registro sin foto de perfil
        When me registro como prestador con correo "prestador@example.com", nombre "Juan", apellido "Pérez" y rubro "Plomería" sin cargar foto de perfil
        Then el sistema me indica que la foto de perfil es obligatoria

    Rule: La foto de perfil debe tener formato png, jpg o jpeg

    Scenario: 03-RPF Rechazar registro con foto de perfil no válida
        When intento cargar una foto de perfil con formato no válido para el registro
        Then el sistema me indica que la foto de perfil no pudo ser cargada

    Rule: La foto de perfil no debe superar los 5MB

    Scenario: 04-RPF Rechazar registro con foto de perfil que supera el tamaño máximo
        When intento cargar una foto de perfil que pesa 6 MB para el registro 
        Then el sistema me indica que la foto de perfil no pudo ser cargada