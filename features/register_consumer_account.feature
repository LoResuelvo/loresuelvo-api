Feature: Registrar cuenta nueva de consumidor
  Como consumidor
  quiero registrarme en la plataforma
  para poder contactar profesionales que resuelvan problemas en mi hogar

    Background:
        Given que no existe un consumidor con correo "ana@example.com"

    Scenario: 01-RCN Registrar una cuenta nueva correctamente
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "Segura12345?"
        Then el sistema confirma el registro

    Rule: El correo electrónico debe tener un formato válido
    
    @wip
    Scenario: 02-RCN Rechazar un registro con correo sin @
        When me registro como usuario consumidor con correo "anaexample.com", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "Segura12345?"
        Then el sistema me indica que el formato del correo es inválido

    @wip
    Scenario: 03-RCN Rechazar un registro con correo sin dominio
        When me registro como usuario consumidor con correo "ana@", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "Segura12345?"
        Then el sistema me indica que el formato del correo es inválido

    @wip
    Scenario: 04-RCN Rechazar un registro con correo sin nombre de usuario
        When me registro como usuario consumidor con correo "@example.com", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "Segura12345?"
        Then el sistema me indica que el formato del correo es inválido

    @wip
    Rule: La contraseña debe contener entre 12 y 64 caracteres
    
    Scenario: 03-RCN Rechazar un registro con contraseña con menos de 12 caracteres
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "abc123"
        Then el sistema me indica que la contraseña es demasiado corta
        
    @wip    
    Rule: La contraseña debe incluir al menos una letra mayúscula, una letra minúscula, un número y un carácter especial
    
    Scenario: 04-RCN Rechazar un registro con contraseña sin mayúscula
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "abc123456789!"
        Then el sistema me indica que la contraseña es insegura

    @wip
    Scenario: 05-RCN Rechazar un registro con contraseña sin minúscula
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "ABC123456789!"
        Then el sistema me indica que la contraseña es insegura

    @wip
    Scenario: 06-RCN Rechazar un registro con contraseña sin número
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana Perez", apellido "Mamani Tipula" y contraseña "aaaaaaasdsadfsaf!"
        Then el sistema me indica que la contraseña es insegura

    Rule: No se puede registrar con un correo electrónico que ya esté en uso

    @wip
    Scenario: 05-RCN Rechazar un registro con correo ya existente
        Given existe un consumidor registrado con correo "carla@example.com"
        When me registro como usuario consumidor con correo "carla@example.com", nombre "Carla Gomez", apellido "Mendiola bondiola" y contraseña "Segura12345?"
        Then el sistema me indica que el correo electrónico ya está registrado
