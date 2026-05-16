Feature: Registrar cuenta nueva de consumidor
  Como consumidor
  quiero registrarme en la plataforma
  para poder contactar profesionales que resuelvan problemas en mi hogar

    Background:
        Given que no existe un consumidor con correo "ana@example.com"

    Scenario: 01-RCN Registrar una cuenta nueva correctamente
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema confirma el registro

    Rule: El correo electrónico debe tener un formato válido
    
    Scenario: 02-RCN Rechazar un registro con correo sin @
        When me registro como usuario consumidor con correo "anaexample.com", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema me indica que el formato del correo es inválido

    Scenario: 03-RCN Rechazar un registro con correo sin dominio
        When me registro como usuario consumidor con correo "ana@", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema me indica que el formato del correo es inválido

    Scenario: 04-RCN Rechazar un registro con correo sin nombre de usuario
        When me registro como usuario consumidor con correo "@example.com", nombre "Ana Perez" y apellido "Mamani Tipula"
        Then el sistema me indica que el formato del correo es inválido

    Rule: No se puede registrar con un correo electrónico que ya esté en uso

    Scenario: 05-RCN Rechazar un registro con correo ya existente
        Given existe un consumidor registrado con correo "carla@example.com"
        When me registro como usuario consumidor con correo "carla@example.com", nombre "Carla Gomez" y apellido "Mendiola bondiola"
        Then el sistema me indica que el correo electrónico ya está registrado
