@US-20
Feature: Conservar el trabajo realizado aunque falle un aviso
    Como usuario de LoResuelvo
    quiero que mis mensajes queden guardados aunque falle el aviso al teléfono
    para poder seguir la conversación al abrir la aplicación

    Scenario: 20.15 Conservar el mensaje cuando no se puede enviar el aviso
        Given que Ana y Juan pueden conversar sobre un trabajo
        And que Juan tiene un teléfono registrado para recibir avisos
        And que el servicio de avisos al teléfono no está disponible
        When Ana le envía un mensaje
        Then LoResuelvo le confirma a Ana que el mensaje fue enviado
        And Juan puede consultar el mensaje en la conversación
