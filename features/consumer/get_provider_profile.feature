Feature: Ver información de perfil de un prestador

  Como consumidor
  quiero consultar la información de perfil de un prestador
  para conocer sus datos disponibles antes de contactarlo

  Background:
    Given que existe el rubro "Plomería"
    And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
    And que estoy autenticado como consumidor "ana@example.com"

  Rule: El perfil presenta la información pública disponible del prestador

    @wip
    Scenario: 01-VPP Consultar el perfil de un prestador
      Given que existe un prestador llamado "Juan Gómez" en el rubro "Plomería" con foto de perfil
      When consulto el perfil del prestador "Juan Gómez"
      Then el sistema devuelve el perfil del prestador
      And el perfil muestra el nombre "Juan" y apellido "Gómez"
      And el perfil muestra la foto del prestador
      And el perfil muestra el rubro "Plomería"

  Rule: El perfil público no expone información privada del prestador

    @wip
    Scenario: 02-VPP Ocultar datos privados del prestador
      Given que existe un prestador llamado "Juan Gómez" en el rubro "Plomería" con foto de perfil
      When consulto el perfil del prestador "Juan Gómez"
      Then el sistema devuelve el perfil del prestador
      And el perfil no expone el correo ni la identidad de autenticación del prestador

  Rule: El prestador consultado debe existir

    @wip
    Scenario: 03-VPP Informar que el prestador no existe
      When consulto el perfil de un prestador inexistente
      Then el sistema informa que el prestador no fue encontrado

  Rule: Solo un usuario autenticado puede consultar perfiles de prestadores

    @wip
    Scenario: 04-VPP Rechazar la consulta sin sesión válida
      Given que existe un prestador llamado "Juan Gómez" en el rubro "Plomería" con foto de perfil
      And que no tengo una sesión válida
      When consulto el perfil del prestador "Juan Gómez"
      Then el sistema deniega el acceso
