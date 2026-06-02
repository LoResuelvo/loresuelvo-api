@wip
Feature: Obtencion de solicitudes de trabajo
    Como usuario
    quiero obtener mis solicitudes pendientes
    para saber su estado y contenido

    Background:
        Given que existe el rubro "Plomería"

    Scenario: 40.1 - OBT obtener solicitudes de trabajo como consumidor pendientes sin solicitudes
        Given que existe un consumidor registrado con correo "consumidor@example.com", nombre "Consumidor" y apellido "Ejemplo"
        And que estoy autenticado como consumidor "consumidor@example.com"
        When obtengo mis solicitudes de trabajo pendientes
        Then el sistema me muestra una lista con 0 solicitudes pendientes

    Scenario: 40.2 - OBT obtener solicitudes de trabajo como consumidor pendientes con solicitudes
        Given que existe un consumidor registrado con correo "consumidor@example.com", nombre "Consumidor" y apellido "Ejemplo"
        And que estoy autenticado como consumidor "consumidor@example.com"
        And que existe una solicitud de trabajo pendiente para el consumidor "consumidor@example.com"
        When obtengo mis solicitudes de trabajo pendientes
        Then el sistema me muestra una lista con 1 solicitudes pendientes

    Scenario: 40.3 - OBT obtener solicitudes de trabajo como prestador pendientes sin solicitudes
        Given existe un prestador registrado con correo "prestador@example.com", nombre "Prestador", apellido "Ejemplo" y rubro "Plomería"
        And que estoy autenticado como prestador "prestador@example.com"
        When obtengo mis solicitudes de trabajo pendientes
        Then el sistema me muestra una lista con 0 solicitudes pendientes

    Scenario: 40.4 - OBT obtener solicitudes de trabajo como prestador pendientes con solicitudes
        Given existe un prestador registrado con correo "prestador@example.com", nombre "Prestador", apellido "Ejemplo" y rubro "Plomería"
        And que estoy autenticado como prestador "prestador@example.com"
        And que existe una solicitud de trabajo pendiente para el prestador "prestador@example.com"
        When obtengo mis solicitudes de trabajo pendientes
        Then el sistema me muestra una lista con 1 solicitudes pendientes
