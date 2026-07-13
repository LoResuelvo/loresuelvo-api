@wip
Feature: Notificar ordenes de trabajo urgentes
    Como usuario
    quiero recibir notificación de mis turnos urgentes
    para estar al tanto y no olvidarme de los mismos

    Rule: Solo se notifica las ordenes de trabajo con vencimiento en menos de 24 horas

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-12T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"

    Scenario: 55.1.1-NWO Notificar ordenes de trabajo con vencimiento en menos de 24 horas
        Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-13T09:00:00-03:00" con la descripción:
            """
            Reparación urgente de una pérdida de agua.
            """
        And que el consumidor "ana@example.com" está disponible para recibir mensajes en tiempo real
        And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
        When el scheduler revisa las órdenes de trabajo urgentes
        Then el consumidor "ana@example.com" recibe la notificación de orden de trabajo urgente
        And el prestador "juan.plomero@example.com" recibe la notificación de orden de trabajo urgente

    Scenario: 55.1.2-NWO No notificar ordenes de trabajo con vencimiento en más de 24 horas
        Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "15000.50" para la fecha y hora "2026-07-13T11:00:00-03:00" con la descripción:
            """
            Reparación programada de una pérdida de agua.
            """
        And que el consumidor "ana@example.com" está disponible para recibir mensajes en tiempo real
        And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
        When el scheduler revisa las órdenes de trabajo urgentes
        Then el consumidor "ana@example.com" no recibe notificaciones de órdenes de trabajo urgentes
        And el prestador "juan.plomero@example.com" no recibe notificaciones de órdenes de trabajo urgentes
