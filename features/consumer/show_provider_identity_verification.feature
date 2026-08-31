Feature: Mostrar la verificación de identidad de un prestador
    Como consumidor
    quiero distinguir a los prestadores con identidad aprobada
    para contratar con mayor información

    Background:
        Given que existe el rubro "Plomería"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"

    Rule: El prestador puede consultar su estado detallado

        Scenario: 59.1 Mostrar como no verificado a un prestador sin sesiones
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When consulto mi perfil de usuario
            Then mi estado de verificación de identidad es "unverified"
            And mi fecha de verificación no está informada

    Rule: El consumidor sólo debe conocer si la identidad está aprobada

        Scenario: 59.2 Mostrar la insignia en la búsqueda de prestadores aprobados
            Given que la identidad de "juan.plomero@example.com" está aprobada
            When busco prestadores del rubro "Plomería"
            Then "juan.plomero@example.com" figura con identidad verificada

        Scenario: 59.3 Mostrar la insignia en el perfil de un prestador aprobado
            Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And que estoy autenticado como consumidor "ana@example.com"
            Given que la identidad de "juan.plomero@example.com" está aprobada
            When consulto el perfil público de "juan.plomero@example.com"
            Then el perfil indica que la identidad está verificada

        @wip
        Scenario: 59.4 Mantener visible a un prestador no aprobado
            Given que la verificación de "juan.plomero@example.com" está en estado "declined"
            When busco prestadores del rubro "Plomería"
            Then "juan.plomero@example.com" continúa apareciendo
            And figura sin identidad verificada

        @wip
        Scenario: 59.5 No exponer detalles privados en el perfil público
            Given que la verificación de "juan.plomero@example.com" fue rechazada por el riesgo "DOCUMENT_EXPIRED"
            When consulto el perfil público de "juan.plomero@example.com"
            Then el perfil no expone el estado detallado de la verificación
            And el perfil no expone códigos de riesgo ni datos de identidad
