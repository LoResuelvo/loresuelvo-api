Feature: Conectar cuenta de Mercado Pago durante el registro de prestador
    Como prestador
    quiero conectar mi cuenta de Mercado Pago durante mi registro
    para quedar habilitado para recibir los pagos de los servicios contratados

    Background:
        Given que existe el rubro "Plomería"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que el registro del prestador "juan.plomero@example.com" está pendiente de conectar una cuenta de Mercado Pago

    Rule: El prestador puede conectar una cuenta de Mercado Pago autorizada

        Scenario: 35.3.1-CMP Conectar correctamente una cuenta de Mercado Pago
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que la cuenta de Mercado Pago "mp-juan" está habilitada para recibir pagos de marketplace
            When inicio la conexión de mi cuenta de Mercado Pago
            And autorizo a LoResuelvo a operar con la cuenta de Mercado Pago "mp-juan"
            Then el sistema confirma la conexión de la cuenta de Mercado Pago
            And la cuenta de Mercado Pago "mp-juan" queda vinculada al prestador "juan.plomero@example.com"
            And el prestador "juan.plomero@example.com" queda habilitado para recibir pagos
            And el prestador "juan.plomero@example.com" queda habilitado para enviar propuestas de servicio

        Scenario: 35.3.2-CMP No duplicar la conexión de una cuenta ya vinculada al mismo prestador
            Given que la cuenta de Mercado Pago "mp-juan" ya está vinculada al prestador "juan.plomero@example.com"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When intento conectar nuevamente la cuenta de Mercado Pago "mp-juan"
            Then el sistema informa que el prestador ya tiene una cuenta de Mercado Pago conectada
            And el sistema conserva una única conexión con la cuenta de Mercado Pago "mp-juan"
            And el prestador "juan.plomero@example.com" permanece habilitado para recibir pagos

    Rule: La conexión es obligatoria para completar la habilitación comercial del prestador

        Scenario: 35.3.3-CMP Mantener incompleta la habilitación mientras no se conecte Mercado Pago
            Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And que existe un chat activo entre el consumidor "ana@example.com" y el prestador "juan.plomero@example.com" con el mensaje inicial:
                """
                Necesito reparar una pérdida de agua en la cocina.
                """
            And que la fecha y hora actual del sistema es "2026-07-20T12:00:00Z"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When intento enviar una propuesta de servicio al consumidor "ana@example.com" sin haber conectado una cuenta de Mercado Pago
            Then el sistema informa que la conexión de Mercado Pago está pendiente
            And la propuesta de servicio no se envía

        @wip
        Scenario: 35.3.4-CMP Conservar el registro cuando el prestador rechaza la autorización
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When inicio la conexión de mi cuenta de Mercado Pago
            And rechazo autorizar a LoResuelvo en Mercado Pago
            Then el sistema no vincula ninguna cuenta de Mercado Pago al prestador
            And el prestador "juan.plomero@example.com" permanece registrado
            And la conexión de Mercado Pago permanece pendiente
            And el sistema permite volver a intentar la conexión

    Rule: Cada cuenta de Mercado Pago sólo puede pertenecer a un prestador de LoResuelvo

        @wip
        Scenario: 35.3.5-CMP Rechazar una cuenta de Mercado Pago vinculada a otro prestador
            Given existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "López" y rubro "Plomería"
            And que la cuenta de Mercado Pago "mp-pedro" está vinculada al prestador "pedro.plomero@example.com"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When intento conectar la cuenta de Mercado Pago "mp-pedro"
            Then el sistema rechaza la conexión de la cuenta de Mercado Pago
            And la cuenta de Mercado Pago "mp-pedro" permanece vinculada solamente al prestador "pedro.plomero@example.com"
            And la conexión de Mercado Pago de "juan.plomero@example.com" permanece pendiente

    Rule: La respuesta de autorización debe corresponder a una conexión iniciada por el prestador

        @wip
        Scenario: 35.3.6-CMP Rechazar una respuesta de autorización con estado de seguridad inválido
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que inicié la conexión de mi cuenta de Mercado Pago
            When Mercado Pago retorna una autorización con un estado de seguridad inválido
            Then el sistema rechaza la respuesta de autorización
            And el sistema no vincula ninguna cuenta de Mercado Pago al prestador
            And la conexión de Mercado Pago permanece pendiente

        @wip
        Scenario: 35.3.7-CMP Rechazar una autorización que ya no puede utilizarse
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que inicié la conexión de mi cuenta de Mercado Pago
            When Mercado Pago retorna un código de autorización "vencido"
            Then el sistema no vincula ninguna cuenta de Mercado Pago al prestador
            And la conexión de Mercado Pago permanece pendiente
            And el sistema permite volver a iniciar la conexión

    Rule: Sólo el prestador autenticado puede iniciar la conexión

        @wip
        Scenario: 35.3.8-CMP Rechazar el inicio de conexión por un consumidor
            Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento iniciar la conexión de una cuenta de Mercado Pago para el prestador "juan.plomero@example.com"
            Then el sistema deniega la conexión de la cuenta de Mercado Pago
            And la conexión de Mercado Pago de "juan.plomero@example.com" permanece pendiente

    Rule: La cuenta de Mercado Pago debe estar habilitada para recibir pagos de marketplace

        @wip
        Scenario: 35.3.9-CMP Rechazar una cuenta que no puede recibir pagos de marketplace
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que la cuenta de Mercado Pago "mp-juan" no está habilitada para recibir pagos de marketplace
            When intento conectar la cuenta de Mercado Pago "mp-juan"
            Then el sistema informa que la cuenta de Mercado Pago no está habilitada para recibir pagos
            And el sistema no vincula la cuenta de Mercado Pago "mp-juan" al prestador
            And la conexión de Mercado Pago permanece pendiente
