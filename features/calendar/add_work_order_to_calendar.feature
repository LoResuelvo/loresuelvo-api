Feature: Agregar una orden de trabajo a Google Calendar
    Como participante de una contratación
    quiero recibir el turno de mi orden de trabajo en mi Google Calendar
    para recordar cuándo y con quién se realizará el servicio

    Scenario: 22.1-AWOC Ambos participantes reciben su propia cita
        Given que la fecha y hora actual del sistema es "2026-08-01T10:00:00Z"
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene Google Calendar conectado
        And el prestador "juan.plomero@example.com" tiene Google Calendar conectado
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then el consumidor "ana@example.com" recibe su cita en su Google Calendar
        And el prestador "juan.plomero@example.com" recibe su cita en su Google Calendar

    Scenario: 22.2-AWOC Sólo el consumidor conectado recibe la cita
        Given que la fecha y hora actual del sistema es "2026-08-01T10:00:00Z"
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene Google Calendar conectado
        And el prestador "juan.plomero@example.com" no tiene Google Calendar conectado
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then el consumidor "ana@example.com" recibe su cita en su Google Calendar
        And el prestador "juan.plomero@example.com" no recibe una cita en Google Calendar

    Scenario: 22.3-AWOC Sólo el prestador conectado recibe la cita
        Given que la fecha y hora actual del sistema es "2026-08-01T10:00:00Z"
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" no tiene Google Calendar conectado
        And el prestador "juan.plomero@example.com" tiene Google Calendar conectado
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then el consumidor "ana@example.com" no recibe una cita en Google Calendar
        And el prestador "juan.plomero@example.com" recibe su cita en su Google Calendar

    @wip
    Scenario: 22.4-AWOC No se crea una cita si ninguno está conectado
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" no tiene Google Calendar conectado
        And el prestador "juan.plomero@example.com" no tiene Google Calendar conectado
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then no se crea ninguna cita en Google Calendar

    @wip
    Scenario: 22.5-AWOC Al conectar aparecen los turnos futuros existentes
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene una orden futura sin exportar
        And el consumidor "ana@example.com" acaba de conectar Google Calendar
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then el consumidor "ana@example.com" recibe la cita futura en su Google Calendar

    @wip
    Scenario: 22.6-AWOC No se agregan turnos pasados
        Given que existe una orden de trabajo pasada para "ana@example.com" y "juan.plomero@example.com" el "2026-07-04T09:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene Google Calendar conectado
        And la fecha y hora actual del sistema es "2026-07-05T10:00:00Z"
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then el consumidor "ana@example.com" no recibe una cita para ese turno

    @wip
    Scenario: 22.7-AWOC La cita muestra horario, duración, contraparte, descripción y privacidad
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene Google Calendar conectado
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then la cita del consumidor "ana@example.com" muestra el horario "2026-08-15T15:00:00Z", dura "90" minutos, identifica a "Juan Gómez", contiene la descripción y es privada

    @wip
    Scenario: 22.8-AWOC Una indisponibilidad de Calendar no impide confirmar la contratación
        Given que existe una propuesta aceptada por "ana@example.com" y "juan.plomero@example.com" para el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene Google Calendar conectado
        And Google Calendar no está disponible
        When se confirma la contratación de la propuesta
        Then la contratación queda confirmada aunque no se pueda crear la cita

    @wip
    Scenario: 22.9-AWOC La cita aparece cuando Calendar vuelve a estar disponible
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene una cita pendiente por una indisponibilidad de Google Calendar
        And Google Calendar volvió a estar disponible
        When se reintenta la sincronización de la cita pendiente
        Then el consumidor "ana@example.com" recibe la cita en su Google Calendar

    @wip
    Scenario: 22.10-AWOC Un turno aparece una sola vez
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene Google Calendar conectado
        When se sincroniza dos veces la misma orden de trabajo con Google Calendar
        Then el consumidor "ana@example.com" conserva una sola cita para ese turno

    @wip
    Scenario: 22.11-AWOC El usuario es informado cuando debe volver a autorizar
        Given que existe una orden de trabajo futura para "ana@example.com" y "juan.plomero@example.com" el "2026-08-15T15:00:00Z" con una duración estimada de "90" minutos y la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And el consumidor "ana@example.com" tiene una conexión de Google Calendar que requiere autorización
        When se sincronizan las órdenes de trabajo futuras con Google Calendar
        Then el consumidor "ana@example.com" recibe un aviso para volver a autorizar Google Calendar
