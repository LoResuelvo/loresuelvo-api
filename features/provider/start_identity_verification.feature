Feature: Iniciar la verificación de identidad de un prestador
    Como prestador registrado
    quiero iniciar mi verificación de identidad
    para demostrar que mi identidad fue comprobada

    Background:
        Given que existe el rubro "Plomería"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"

    Rule: Sólo un prestador registrado puede iniciar su propia verificación

        Scenario: 58.1 Iniciar correctamente una verificación de identidad
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When inicio mi verificación de identidad
            Then el sistema entrega las credenciales temporales de la verificación
            And la verificación de "juan.plomero@example.com" queda en estado "not_started"
            And la sesión queda asociada solamente al prestador "juan.plomero@example.com"

        Scenario: 58.2 Rechazar el inicio de verificación por un consumidor
            Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento iniciar la verificación de identidad del prestador "juan.plomero@example.com"
            Then el sistema deniega el inicio de la verificación
            And no se crea ninguna sesión para "juan.plomero@example.com"

    Rule: Una sesión activa debe reutilizarse

        Scenario: 58.3 Reutilizar una sesión de verificación activa
            Given que la verificación de "juan.plomero@example.com" está en estado "in_progress"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When inicio nuevamente mi verificación de identidad
            Then el sistema entrega las credenciales de la misma sesión
            And se conserva una única sesión activa para "juan.plomero@example.com"

        Scenario: 58.4 No crear otra sesión para un prestador aprobado
            Given que la identidad de "juan.plomero@example.com" está aprobada
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When intento iniciar otra verificación de identidad
            Then el sistema informa que mi identidad ya está verificada
            And no se crea una nueva sesión

    Rule: El registro del prestador no depende de la disponibilidad del verificador

        @wip
        Scenario: 58.5 Conservar el registro cuando el verificador no está disponible
            Given que el verificador de identidad no está disponible
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When inicio mi verificación de identidad
            Then el sistema informa que la verificación no está disponible temporalmente
            And el prestador "juan.plomero@example.com" permanece registrado
            And no se guarda una sesión incompleta
