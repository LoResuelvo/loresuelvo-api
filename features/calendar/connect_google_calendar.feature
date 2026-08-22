Feature: Vincular Google Calendar
    Como usuario de LoResuelvo
    quiero vincular mi Google Calendar
    para que el sistema pueda usarlo en funcionalidades futuras

    Scenario: 57.1-CGC Un consumidor vincula Calendar desde la web
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que estoy autenticado como consumidor "ana@example.com"
        When inicio la vinculación de Google Calendar desde la web
        Then el sistema devuelve una autorización web de Google Calendar
        And la autorización solicita el permiso de eventos propios de Google Calendar
        When autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar
        And consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "connected"

    Scenario: 57.2-CGC Un prestador vincula Calendar desde la web
        Given que existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y la foto de perfil cargada
        And que estoy autenticado como prestador "juan.plomero@example.com"
        When inicio la vinculación de Google Calendar desde la web
        Then el sistema devuelve una autorización web de Google Calendar
        And la autorización solicita el permiso de eventos propios de Google Calendar
        When autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar
        And consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "connected"

    Scenario: 57.3-CGC Un usuario vincula Calendar desde Android
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que estoy autenticado como consumidor "ana@example.com"
        When vinculo Google Calendar desde Android con el server auth code "android-calendar-code"
        Then el sistema confirma la vinculación de Google Calendar
        And consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "connected"

    @wip
    Scenario: 57.4-CGC El usuario rechaza la autorización y permanece desconectado
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que estoy autenticado como consumidor "ana@example.com"
        When inicio la vinculación de Google Calendar desde la web
        And rechazo autorizar el acceso de Google Calendar
        Then el sistema informa que la autorización de Google Calendar fue rechazada
        And consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "disconnected"

    @wip
    Scenario: 57.5-CGC Una nueva vinculación no duplica una conexión activa
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que el consumidor "ana@example.com" ya tiene Google Calendar vinculado
        And que estoy autenticado como consumidor "ana@example.com"
        When vuelvo a vincular Google Calendar desde la web
        And autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar
        And el consumidor "ana@example.com" conserva una única conexión de Google Calendar
        And consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "connected"

    @wip
    Scenario: 57.6-CGC Un usuario vuelve a autorizar una conexión que requiere atención
        Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez" sin foto de perfil
        And que la conexión de Google Calendar de "ana@example.com" requiere atención
        And que estoy autenticado como consumidor "ana@example.com"
        When vuelvo a autorizar Google Calendar desde la web
        And autorizo el acceso de Google Calendar
        Then el sistema confirma la vinculación de Google Calendar
        And consulto mi información de usuario autenticado
        Then el perfil informa el estado de Google Calendar "connected"
