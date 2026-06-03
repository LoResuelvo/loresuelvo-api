Feature: 46 Aceptar solicitud de trabajo

Como prestador
quiero aceptar una solicitud de trabajo
para poder iniciar una chat formal con el consumidor

Background: 
    Given que existe el rubro "Plomería"

Scenario: Aceptar solicitud pendiente como prestador destinatario
  Given que existe una solicitud de trabajo pendiente para el prestador "prestador@example.com"
  And que estoy autenticado como prestador "prestador@example.com"
  When acepto la solicitud de trabajo pendiente
  Then la solicitud de trabajo queda aceptada
  And la conversación vinculada queda activa
  And el prestador puede enviar un mensaje en el chat vinculado

Scenario: No permitir aceptar solicitud a otro prestador
  Given que existe una solicitud de trabajo pendiente para el prestador "prestador@example.com"
  And existe un prestador registrado con correo "otro-prestador@example.com", nombre "Otro", apellido "Prestador" y rubro "Plomería"
  And que estoy autenticado como prestador "otro-prestador@example.com"
  When intento aceptar la solicitud de trabajo pendiente
  Then el sistema deniega la aceptación de la solicitud

Scenario: Consumidor puede superar el límite de mensajes luego de aceptación
  Given que existe una solicitud de trabajo pendiente aceptada
  And que estoy autenticado como consumidor "consumidor@example.com"
  When envío seis mensajes en el chat vinculado
  Then el sistema registra los seis mensajes
