Feature: Registrar consumidor con dirección
  Como consumidor
  quiero registrar obligatoriamente el domicilio donde solicitaré los servicios
  para recibir recomendaciones de prestadores que trabajen en mi zona

    Background:
        Given que no existe un usuario con correo "ana@example.com"
        And que están habilitadas las zonas de cobertura "Comuna 6", "Comuna 1" y "Comuna 2"

    Rule: La dirección es obligatoria y debe identificar un domicilio preciso

    Scenario: 1.2.1-RCWA Registrar una cuenta con calle y número
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" indicando como domicilio "Av. Rivadavia 5100"
        Then el sistema confirma el registro
        And el domicilio del consumidor conserva la calle "Av. Rivadavia" y el número "5100"
        And el domicilio del consumidor tiene coordenadas válidas
        And el domicilio del consumidor queda asociado a la zona de cobertura "Comuna 6"

    Scenario: 1.2.2-RCWA Registrar una cuenta con piso y departamento
        When me registro como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" indicando como domicilio "Av. Rivadavia 5100", piso "4" y departamento "B"
        Then el sistema confirma el registro
        And el domicilio del consumidor conserva el piso "4" y el departamento "B"
        And el domicilio del consumidor queda asociado a la zona de cobertura "Comuna 6"

    Scenario: 1.2.3-RCWA Rechazar un registro sin dirección
        When intento registrarme como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" sin indicar un domicilio
        Then el sistema me indica que la dirección es obligatoria
        And no existe un usuario con correo "ana@example.com"

    Scenario: 1.2.4-RCWA Rechazar un registro sin calle
        When intento registrarme como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" indicando solamente el número "5100"
        Then el sistema me indica que la calle es obligatoria
        And no existe un usuario con correo "ana@example.com"

    Scenario: 1.2.5-RCWA Rechazar un registro sin número
        When intento registrarme como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" indicando solamente la calle "Av. Rivadavia"
        Then el sistema me indica que el número es obligatorio
        And no existe un usuario con correo "ana@example.com"

    Scenario: 1.2.6-RCWA Rechazar una dirección que no puede geolocalizarse
        When intento registrarme como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" indicando un domicilio inexistente
        Then el sistema me indica que no pudo validar la dirección
        And no existe un usuario con correo "ana@example.com"

    Rule: El domicilio debe pertenecer a una zona de cobertura habilitada

    Scenario: 1.2.7-RCWA Rechazar un domicilio fuera del mercado habilitado
        When intento registrarme como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" indicando el domicilio "Av. Maipú 100, Vicente López"
        Then el sistema me indica que todavía no ofrece servicios en esa ubicación
        And no existe un usuario con correo "ana@example.com"

    Rule: El registro debe ser atómico si no puede resolverse la ubicación

    Scenario: 1.2.8-RCWA No registrar la cuenta cuando el servicio de ubicación no está disponible
        Given que el servicio de resolución de ubicación no está disponible
        When intento registrarme como usuario consumidor con correo "ana@example.com", nombre "Ana" y apellido "Perez" indicando como domicilio "Av. Rivadavia 5100"
        Then el sistema me indica que no puede validar la dirección temporalmente
        And no existe un usuario con correo "ana@example.com"
