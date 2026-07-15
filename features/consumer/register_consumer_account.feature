Feature: Registrar cuenta nueva de consumidor
  Como consumidor
  quiero registrarme en la plataforma
  para poder contactar profesionales que resuelvan problemas en mi hogar

    Background:
        Given que no existe un usuario con correo "ana@example.com"

    Scenario: 01-RCN Registrar una cuenta nueva con foto de perfil
        Given que cargué una foto de perfil válida para mi registro como consumidor
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema confirma el registro
        And el registro del consumidor incluye su foto de perfil

    Scenario: 02-RCN Registrar una cuenta nueva sin foto de perfil
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez" y apellido "Mamani Tipula" sin cargar foto de perfil
        Then el sistema confirma el registro
        And el registro del consumidor no incluye una foto de perfil

    Rule: La foto de perfil, cuando se proporciona, debe estar disponible para el usuario

    Scenario: 03-RCN Rechazar el registro con una foto de perfil no disponible
        When intento registrarme como consumidor utilizando una foto de perfil no disponible
        Then el sistema me indica que la foto de perfil no pudo ser cargada

    Rule: La foto de perfil debe tener formato png, jpg, jpeg o webp

    Scenario: 04-RCN Rechazar una foto de perfil con formato no válido
        When intento cargar una foto de perfil con formato no válido para el registro
        Then el sistema me indica que la foto de perfil no pudo ser cargada

    Rule: La foto de perfil no debe superar los 5 MB

    Scenario: 05-RCN Rechazar una foto de perfil que supera el tamaño máximo
        When intento cargar una foto de perfil que pesa 6 MB para el registro
        Then el sistema me indica que la foto de perfil no pudo ser cargada

    Rule: El correo electrónico debe tener un formato válido
    
    Scenario: 06-RCN Rechazar un registro con correo sin @
        When me registro como usuario consumidor con correo "anaexample.com", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema me indica que el formato del correo es inválido

    Scenario: 07-RCN Rechazar un registro con correo sin dominio
        When me registro como usuario consumidor con correo "ana@", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema me indica que el formato del correo es inválido

    Scenario: 08-RCN Rechazar un registro con correo sin nombre de usuario
        When me registro como usuario consumidor con correo "@example.com", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema me indica que el formato del correo es inválido

    Rule: No se puede registrar con un correo electrónico que ya esté en uso

    Scenario: 09-RCN Rechazar un registro con correo ya existente
        Given existe un consumidor registrado con correo "carla@example.com"
        When me registro como usuario consumidor con correo "carla@example.com", nombre "Carla Gomez" y apellido "Mendiola bondiola"
        Then el sistema me indica que el correo electrónico ya está registrado
