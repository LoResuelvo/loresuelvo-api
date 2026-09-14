Feature: Obtener el perfil del administrador autenticado

  Como administrador
  quiero consultar mi perfil después de autenticarme
  para identificar mi cuenta en la plataforma Admin

  Rule: El administrador debe tener un perfil local vinculado a su identidad autenticada

    Scenario: 3.2.1-LA Obtener el perfil del administrador provisionado
      Given que existe un administrador provisionado con correo "admin@example.com", nombre "Ana" y apellido "Pérez"
      And que estoy autenticado como administrador "admin@example.com"
      When consulto mi información de usuario autenticado
      Then el sistema devuelve mi perfil de administrador
      And el perfil contiene el nombre "Ana", apellido "Pérez" y correo "admin@example.com"
      And el perfil informa el rol "admin"

    Scenario: 3.2.2-LA Rechazar la consulta sin autenticación válida
      Given que no tengo una sesión válida
      When consulto mi información de usuario autenticado
      Then el sistema deniega el acceso

    Scenario: 3.2.3-LA No obtener un perfil sin aprovisionamiento local
      Given que estoy autenticado con una identidad que no pertenece a un usuario registrado
      When consulto mi información de usuario autenticado
      Then el sistema informa que el usuario no fue encontrado
