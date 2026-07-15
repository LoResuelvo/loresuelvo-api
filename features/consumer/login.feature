Feature: Obtener la información del usuario autenticado

  Como usuario registrado
  quiero consultar mi perfil autenticado
  para recuperar toda la información correspondiente a mi tipo de cuenta

  Rule: La respuesta contiene la información común de cualquier usuario

    Scenario: 01-ME Obtener el perfil completo de un consumidor sin foto
      Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
      And que estoy autenticado como consumidor "ana@example.com"
      When consulto mi información de usuario autenticado
      Then el sistema devuelve mi perfil de consumidor
      And el perfil contiene el nombre "Ana", apellido "Pérez" y correo "ana@example.com"
      And el perfil informa el rol "consumer"
      And el perfil no incluye una foto de perfil

    Scenario: 02-ME Obtener el perfil completo de un consumidor con foto
      Given que cargué una foto de perfil válida para mi registro como consumidor
      And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" con la foto de perfil cargada
      And que estoy autenticado como consumidor "ana@example.com"
      When consulto mi información de usuario autenticado
      Then el sistema devuelve mi perfil de consumidor
      And el perfil contiene el nombre "Ana", apellido "Pérez" y correo "ana@example.com"
      And el perfil informa el rol "consumer"
      And el perfil incluye la foto de perfil

  Rule: La respuesta contiene la información específica del prestador

    Scenario: 03-ME Obtener el perfil completo de un prestador
      Given que existe el rubro "Plomería"
      And que cargué una foto de perfil válida para mi registro como prestador
      And que existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y la foto de perfil cargada
      And que estoy autenticado como prestador "juan@example.com"
      When consulto mi información de usuario autenticado
      Then el sistema devuelve mi perfil de prestador
      And el perfil contiene el nombre "Juan", apellido "Gómez" y correo "juan@example.com"
      And el perfil informa el rol "provider"
      And el perfil incluye la foto de perfil
      And el perfil incluye el rubro "Plomería"

  Rule: Solo una identidad autenticada y registrada puede consultar su perfil

    Scenario: 04-ME Rechazar una consulta sin sesión válida
      Given que no tengo una sesión válida
      When consulto mi información de usuario autenticado
      Then el sistema deniega el acceso

    Scenario: 05-ME Informar que el usuario autenticado no está registrado
      Given que estoy autenticado con una identidad que no pertenece a un usuario registrado
      When consulto mi información de usuario autenticado
      Then el sistema informa que el usuario no fue encontrado
