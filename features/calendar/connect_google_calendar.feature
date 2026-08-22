Feature: Vincular Google Calendar
    Como usuario de LoResuelvo
    quiero vincular mi Google Calendar
    para que el sistema pueda usarlo en funcionalidades futuras

    Scenario: 57.1-CGC Un consumidor inicia la vinculación de Calendar desde la web
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que estoy autenticado como consumidor "ana@example.com"
        When inicio la vinculación de Google Calendar desde la web
        Then el sistema devuelve una autorización web de Google Calendar
        And la autorización solicita el permiso de eventos propios de Google Calendar

    Scenario: 57.1.1-CGC Un consumidor completa la vinculación de Calendar desde la web
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que estoy autenticado como consumidor "ana@example.com"
        And que existe una autorización web pendiente de Google Calendar
        When autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar

    Scenario: 57.2-CGC Un prestador inicia la vinculación de Calendar desde la web
        Given que existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y la foto de perfil cargada
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When inicio la vinculación de Google Calendar desde la web
        Then el sistema devuelve una autorización web de Google Calendar
        And la autorización solicita el permiso de eventos propios de Google Calendar

    Scenario: 57.2.1-CGC Un prestador completa la vinculación de Calendar desde la web
        Given que existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y la foto de perfil cargada
        And que estoy autenticado como prestador "juan.plomero@example.com"
        And que existe una autorización web pendiente de Google Calendar
        When autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar

    Scenario: 57.3-CGC Un usuario vincula Calendar desde Android
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que estoy autenticado como consumidor "ana@example.com"
        When vinculo Google Calendar desde Android con el server auth code "android-calendar-code"
        Then el sistema confirma la vinculación de Google Calendar

    Scenario: 57.4-CGC El usuario rechaza la autorización de Calendar
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que estoy autenticado como consumidor "ana@example.com"
        And que existe una autorización web pendiente de Google Calendar
        When rechazo autorizar el acceso de Google Calendar
        Then el sistema informa que la autorización de Google Calendar fue rechazada

    Scenario: 57.4.1-CGC El usuario permanece desconectado después de rechazar Calendar
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que el usuario "ana@example.com" rechazó la autorización de Google Calendar
        And que estoy autenticado como consumidor "ana@example.com"
        When consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "disconnected"

    Scenario: 57.5-CGC Una nueva vinculación no duplica una conexión activa
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que el consumidor "ana@example.com" ya tiene Google Calendar vinculado
        And que estoy autenticado como consumidor "ana@example.com"
        And que existe una autorización web pendiente de Google Calendar
        When autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar
        And el consumidor "ana@example.com" conserva una única conexión de Google Calendar

    Scenario: 57.6-CGC Un usuario vuelve a autorizar una conexión que requiere atención
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que la conexión de Google Calendar de "ana@example.com" requiere atención
        And que estoy autenticado como consumidor "ana@example.com"
        And que existe una autorización web pendiente de Google Calendar
        When autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar
        And la conexión de Google Calendar queda en estado "connected"

    Scenario: 57.6.1-CGC El perfil informa una conexión activa de Calendar
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que el consumidor "ana@example.com" ya tiene Google Calendar vinculado
        And que estoy autenticado como consumidor "ana@example.com"
        When consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "connected"
