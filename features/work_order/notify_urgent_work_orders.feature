@wip
Feature: Notificar ordenes de trabajo urgentes
    Como usuario
    quiero recibir notificación de mis turnos urgentes
    para estar al tanto y no olvidarme de los mismos

    Rule: Solo se notifica las ordenes de trabajo con vencimiento en menos de 24 horas

    Scenario: 55.1.1-NWO Notificar ordenes de trabajo con vencimiento en menos de 24 horas 
    
    Scenario: 55.1.2-NWO No notificar ordenes de trabajo con vencimiento en más de 24 horas
        