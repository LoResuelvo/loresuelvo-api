@wip
Feature: Consultar la ficha y el historial administrativo de un consumidor
    Como operador autorizado
    quiero consultar la ficha y los recursos de contratación de un consumidor
    para investigar una consulta sin confundir datos actuales con evidencia histórica

    Background:
        Given que existe el rubro "Plomería"
        And que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14"
        And que existen los siguientes consumidores registrados:
            | correo              | nombre  | apellido |
            | ana@example.com     | Ana     | Pérez    |
            | beatriz@example.com | Beatriz | Suárez   |
        And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "luis@example.com", nombre "Luis", apellido "Ruiz" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And que existe un administrador provisionado con correo "soporte@example.com", nombre "Sofía" y apellido "López"
        And que estoy autenticado como administrador "soporte@example.com" con el permiso "read:consumers"

    Rule: La ficha muestra datos disponibles sin inventar evidencia

        Scenario: 61.2.1-CH Entregar la ficha y auditar su acceso antes de responder
            Given que el consumidor "ana@example.com" tiene la dirección actual "Av. Rivadavia", número "5100", piso "4", unidad "B"
            And que el domicilio del consumidor "ana@example.com" pertenece a la zona de cobertura "Comuna 6"
            And que "ana@example.com" no tiene foto de perfil persistida ni actividad de contratación
            When consulto la ficha y el historial administrativo de "ana@example.com" con correlación "consumer-history-1"
            Then el sistema responde con estado 200
            And la ficha identifica al consumidor por su ID persistido, rol, nombre, apellido, correo y fecha de registro
            And la foto de perfil se informa como nula
            And la dirección informa "Av. Rivadavia", "5100", "4" y "B", con procedencia "current_consumer_profile"
            And la zona informa el ID persistido, nombre "Comuna 6" y disponibilidad habilitada, con procedencia "current_consumer_profile"
            And el resumen global informa 0 solicitudes, 0 propuestas y 0 órdenes
            And la página informa una colección vacía, no nula, límite 20 y cursor siguiente nulo
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"
            And antes de entregar la ficha queda preparado exactamente un evento de acceso al recurso "consumer" con el ID persistido de "ana@example.com", el operador "soporte@example.com" y la correlación "consumer-history-1"
            And el resultado del evento es "prepared", sin contenido de la ficha ni motivo manual

    Rule: El historial conserva cada recurso, sus relaciones y la referencia de operación
        Background:
            Given que la fecha y hora actual del sistema es "2026-09-28T00:00:00Z"
            And que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor          | prestador         | creada               | estado   |
                | S1        | ana@example.com     | juan@example.com  | 2026-09-20T10:00:00Z | accepted |
                | S2        | ana@example.com     | luis@example.com  | 2026-09-21T10:00:00Z | accepted |
                | S3        | ana@example.com     | pedro@example.com | 2026-09-22T10:00:00Z | pending  |
                | S4        | beatriz@example.com | juan@example.com  | 2026-09-23T10:00:00Z | pending  |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada               | fecha programada     | duración | estado   |
                | P1        | S1        | 2026-09-20T11:00:00Z | 2026-10-02T13:00:00Z | 60       | accepted |
                | P2        | S1        | 2026-09-20T12:00:00Z | 2026-10-03T13:00:00Z | 60       | rejected |
                | P3        | S1        | 2026-09-20T13:00:00Z | 2026-10-04T13:00:00Z | 60       | pending  |
                | P4        | S2        | 2026-09-21T11:00:00Z | 2026-09-24T13:00:00Z | 60       | accepted |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada             | estado           | finalización informada | saldo pagado |
                | O1    | P1        | 2026-09-20T14:00:00Z | scheduled        |                        |              |
                | O4    | P4        | 2026-09-21T14:00:00Z | awaiting_payment | 2026-09-24T15:00:00Z   |              |

        Scenario: 61.2.2-CH Mostrar solicitudes sin etapas posteriores y varias propuestas u órdenes
            When consulto la ficha y el historial administrativo de "ana@example.com"
            Then el sistema responde con estado 200
            And el resumen global informa 3 solicitudes, 4 propuestas y 2 órdenes
            And el historial contiene exactamente, en este orden:
                | tipo             | recurso | estado           | prestador         | instante             |
                | job_request      | S3      | pending          | pedro@example.com | 2026-09-22T10:00:00Z |
                | work_order       | O4      | awaiting_payment | luis@example.com  | 2026-09-21T14:00:00Z |
                | service_proposal | P4      | accepted         | luis@example.com  | 2026-09-21T11:00:00Z |
                | job_request      | S2      | accepted         | luis@example.com  | 2026-09-21T10:00:00Z |
                | work_order       | O1      | scheduled        | juan@example.com  | 2026-09-20T14:00:00Z |
                | service_proposal | P3      | pending          | juan@example.com  | 2026-09-20T13:00:00Z |
                | service_proposal | P2      | rejected         | juan@example.com  | 2026-09-20T12:00:00Z |
                | service_proposal | P1      | accepted         | juan@example.com  | 2026-09-20T11:00:00Z |
                | job_request      | S1      | accepted         | juan@example.com  | 2026-09-20T10:00:00Z |
            And cada recurso conserva su ID persistido y el ID, nombre y apellido del prestador correspondiente
            And las propuestas y órdenes conservan exactamente estas relaciones por IDs persistidos:
                | recurso | solicitud | propuesta |
                | P1      | S1        |           |
                | P2      | S1        |           |
                | P3      | S1        |           |
                | P4      | S2        |           |
                | O1      | S1        | P1        |
                | O4      | S2        | P4        |
            And las entradas de solicitud no incluyen un vínculo singular a propuesta u orden
            And las solicitudes informan su created_on persistido y las propuestas su created_on, scheduled_on, estimated_duration_minutes y booking_payment_deadline persistidos, con zona horaria en las fechas
            And las órdenes informan accepted_on, completion_reported_on y balance_paid_on desde sus datos persistidos, con zona horaria en las fechas disponibles
            And "O4" informa completion_reported_on "2026-09-24T15:00:00Z" y balance_paid_on nulo
            And "O1" informa completion_reported_on y balance_paid_on nulos
            And no se incorporan campos de aceptación de solicitud o propuesta sin un instante propio persistido
            And "S3" referencia la operación "jr-" seguida de su ID persistido
            And "S1", "P1" y "O1" referencian la operación "jr-" seguida del ID persistido de "S1"
            And "P2" y "P3" referencian cada una su operación "sp-" seguida de su propio ID persistido
            And "P4" y "O4" referencian la operación "jr-" seguida del ID persistido de "S2"
            And no aparece ningún recurso de "beatriz@example.com"

        Scenario: 61.2.3-CH No atribuir el domicilio actual a contrataciones anteriores
            Given que el consumidor "ana@example.com" tiene la dirección actual "Av. Corrientes", número "6200", piso "2", unidad "A"
            When consulto la ficha y el historial administrativo de "ana@example.com"
            Then el sistema responde con estado 200
            And la dirección informa "Av. Corrientes", "6200", "2" y "A", con procedencia "current_consumer_profile"
            And las entradas no contienen un campo de domicilio histórico ni copian la dirección del perfil

        Scenario Outline: 61.2.4-CH Filtrar estados compatibles con el tipo de recurso
            When consulto el historial de "ana@example.com" con tipo "<tipo>" y estado "<estado>"
            Then el sistema responde con estado 200
            And la página contiene exactamente los recursos "<recursos>"
            And el resumen global informa 3 solicitudes, 4 propuestas y 2 órdenes

            Examples:
                | tipo             | estado           | recursos |
                | job_request      | pending          | S3       |
                | service_proposal | rejected         | P2       |
                | work_order       | scheduled        | O1       |
                | work_order       | awaiting_payment | O4       |
                | work_order       | paid             | ninguno  |

        Scenario: 61.2.5-CH Combinar prestador y ventana temporal del recurso
            When consulto el historial del consumidor "ana@example.com" filtrado por el prestador "juan@example.com" desde "2026-09-20T11:00:00Z" hasta "2026-09-20T14:00:00Z"
            Then el sistema responde con estado 200
            And la página contiene exactamente los recursos "P3, P2, P1" en ese orden

        Scenario: 61.2.6-CH Entregar una página acotada con cursor siguiente
            When consulto el historial de "ana@example.com" con límite 2
            Then el sistema responde con estado 200
            And la página contiene exactamente los recursos "S3, O4" en ese orden
            And la página informa límite 2 y un cursor siguiente no nulo
            And el resumen global informa 3 solicitudes, 4 propuestas y 2 órdenes

        Scenario: 61.2.7-CH Continuar la página sin repetir recursos
            Given que consulté la primera página del historial de "ana@example.com" con límite 2 y guardé su cursor
            When continúo el historial de "ana@example.com" con el cursor guardado y límite 2
            Then el sistema responde con estado 200
            And la página contiene exactamente los recursos "P4, S2" en ese orden
            And la página informa un cursor siguiente no nulo

        Scenario: 61.2.8-CH Minimizar el resumen sin eludir permisos de detalle, chat ni pagos
            Given que la conversación de "S1" tiene mensajes entre "ana@example.com" y "juan@example.com"
            And que "S1" tiene una imagen adjunta
            When consulto la ficha y el historial administrativo de "ana@example.com"
            Then el sistema responde con estado 200
            And la ficha y cada tipo de entrada contienen únicamente los campos del contrato público acotado
            And el texto de los mensajes y los IDs de los adjuntos preparados no aparecen en la respuesta
            And la respuesta no contiene credenciales, identificadores de autenticación ni documentos privados
            And no se devuelven transacciones financieras completas ni payloads del procesador
            And las referencias de operación señalan que el detalle requiere "read:admin_operations", el chat "read:admin_chat_audit", independientemente de "read:consumers"

    Rule: La autorización y los errores no se convierten en fichas vacías

        Scenario Outline: 61.2.9-CH Rechazar combinaciones de consulta inválidas
            When consulto el historial de "ana@example.com" con la consulta "<consulta>"
            Then el sistema responde con estado 400
            And no se entrega la ficha ni el historial
            And no se registra ningún evento de acceso para esta solicitud

            Examples:
                | consulta                                                     |
                | type=unknown                                                 |
                | status=pending                                               |
                | type=job_request&status=rejected                             |
                | type=service_proposal&status=paid                             |
                | limit=0                                                      |
                | limit=101                                                    |
                | provider_id=0                                                |
                | from=2026-09-22T10:00:00Z&to=2026-09-20T10:00:00Z             |
                | from=2026-09-20                                               |
                | cursor=invalid                                               |

        Scenario Outline: 61.2.10-CH Rechazar acceso sin JWT o sin read:consumers
            Given que consulto con autenticación "<autenticación>"
            When consulto la ficha y el historial administrativo de "ana@example.com"
            Then el sistema responde con estado <estado>
            And no se entrega la ficha ni el historial
            And no se registra ningún evento de acceso para esta solicitud

            Examples:
                | autenticación                    | estado |
                | sin token                        | 401    |
                | token inválido                   | 401    |
                | soporte con read:admin_operations | 403    |

        Scenario: 61.2.11-CH Informar un consumidor inexistente
            When consulto la ficha y el historial administrativo con un ID positivo de consumidor inexistente
            Then el sistema responde con estado 404
            And no se entrega la ficha ni el historial
            And no se registra ningún evento de acceso para esta solicitud

        Scenario: 61.2.12-CH Fallar cerrado si no se puede registrar el acceso
            Given que falla la persistencia del evento de acceso administrativo
            When consulto la ficha y el historial administrativo de "ana@example.com"
            Then el sistema responde con estado 500
            And no se entrega la ficha ni el historial
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"
            And no se persiste ningún evento de acceso para esta solicitud
