@US-20
Feature: Recibir mis avisos en los teléfonos donde uso mi cuenta
    Como consumidor o prestador
    quiero recibir los avisos que me corresponden en mis teléfonos
    para seguir mis servicios sin recibir avisos de otras cuentas

    Background:
        Given que Ana es una consumidora y Juan es un prestador registrados
        And que Ana y Juan pueden conversar sobre un trabajo

    Scenario: 20.12 Avisar en todos los teléfonos registrados de una persona
        Given que Ana registró dos teléfonos para recibir avisos de su cuenta
        When Juan le envía un mensaje
        Then LoResuelvo envía el aviso de nuevo mensaje a los dos teléfonos de Ana

    Scenario: 20.13 Dejar de enviar avisos a un teléfono desvinculado
        Given que Ana registró dos teléfonos para recibir avisos de su cuenta
        And que desvinculó uno de ellos
        When Juan le envía un mensaje
        Then LoResuelvo envía el aviso solamente al teléfono que sigue registrado

    Scenario: 20.14 Recibir solo los avisos de la cuenta actual después de cambiar de cuenta
        Given que Ana tenía un teléfono registrado para recibir sus avisos
        And que Carla inició sesión en ese mismo teléfono y lo registró para su cuenta
        When Juan le envía un mensaje a Ana
        Then LoResuelvo no envía el aviso de Ana al teléfono que ahora usa Carla
