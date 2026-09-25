Feature: Consultar la bandeja administrativa de contrataciones
    Como operador de LoResuelvo
    quiero consultar una bandeja transversal de contrataciones y sus cuellos de botella
    para identificar qué servicios requieren atención, en qué etapa están y quién debe actuar

    Background:
        Given que existen los rubros "Plomería" y "Electricidad"
        And que existen los siguientes consumidores registrados:
            | correo              | nombre  | apellido |
            | ana@example.com     | Ana     | Pérez    |
            | carla@example.com   | Carla   | Gómez    |
            | beatriz@example.com | Beatriz | Suárez   |
        And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "luis@example.com", nombre "Luis", apellido "Ruiz" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro@example.com", nombre "Pedro", apellido "Dib" y rubro "Electricidad"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: Cada solicitud, propuesta y orden aparece en exactamente una operación con identidad estable

        Background:
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"

        Scenario: 64.1-AO Incluir cada etapa sin correlacionar conversaciones por consumidor o fecha
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor        | prestador         | creada                    | estado   |
                | S1        | ana@example.com   | juan@example.com  | 2026-09-25T09:00:00-03:00 | pending  |
                | S2        | carla@example.com | juan@example.com  | 2026-09-25T08:00:00-03:00 | accepted |
                | S3        | ana@example.com   | pedro@example.com | 2026-09-25T08:30:00-03:00 | accepted |
                | S4        | carla@example.com | pedro@example.com | 2026-09-19T10:00:00-03:00 | accepted |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada                    | fecha programada          | duración | estado   |
                | P3        | S3        | 2026-09-25T09:00:00-03:00 | 2026-09-28T10:00:00-03:00 | 60       | pending  |
                | P4        | S4        | 2026-09-20T10:00:00-03:00 | 2026-09-27T10:00:00-03:00 | 60       | accepted |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada                  | estado    | finalización informada | saldo pagado |
                | O4    | P4        | 2026-09-20T12:00:00-03:00 | scheduled |                        |              |
            When consulto la bandeja administrativa de contrataciones
            Then el sistema responde con estado 200
            And la bandeja contiene exactamente las siguientes operaciones:
                | operación | solicitud | propuesta | orden | etapa                |
                | S1        | S1        |           |       | request_pending      |
                | S2        | S2        |           |       | request_accepted     |
                | S3        | S3        | P3        |       | proposal_pending     |
                | S4        | S4        | P4        | O4    | work_order_scheduled |
            And cada referencia ausente se informa explícitamente como nula

        Scenario: 64.2-AO Conservar el identificador de la operación al avanzar de etapa
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor      | prestador        | creada                    | estado  |
                | S1        | ana@example.com | juan@example.com | 2026-09-22T10:00:00-03:00 | pending |
            And que consulté la bandeja y registré el identificador de la operación "S1"
            And que luego "S1" fue aceptada, recibió la propuesta "P1" y la seña de "P1" fue aprobada generando la orden "O1"
            When consulto la bandeja administrativa de contrataciones
            Then la bandeja contiene exactamente las siguientes operaciones:
                | operación | solicitud | propuesta | orden | etapa                |
                | S1        | S1        | P1        | O1    | work_order_scheduled |
            And la operación "S1" conserva el identificador registrado

        Scenario: 64.3-AO Distinguir varias propuestas de una conversación sin duplicar intentos de pago
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor      | prestador        | creada                    | estado   |
                | S1        | ana@example.com | juan@example.com | 2026-09-01T10:00:00-03:00 | accepted |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada                    | fecha programada          | duración | estado   |
                | P1        | S1        | 2026-09-02T10:00:00-03:00 | 2026-09-10T10:00:00-03:00 | 60       | accepted |
                | P2        | S1        | 2026-09-20T10:00:00-03:00 | 2026-09-30T10:00:00-03:00 | 60       | pending  |
            And que la seña de "P1" tuvo un intento de pago rechazado antes del intento aprobado
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada                  | estado | finalización informada    | saldo pagado              |
                | O1    | P1        | 2026-09-03T10:00:00-03:00 | paid   | 2026-09-10T12:00:00-03:00 | 2026-09-10T15:00:00-03:00 |
            When consulto la bandeja administrativa de contrataciones
            Then la bandeja contiene exactamente las siguientes operaciones:
                | operación | solicitud | propuesta | orden | etapa            |
                | S1        | S1        | P1        | O1    | work_order_paid  |
                | P2        | S1        | P2        |       | proposal_pending |
            And las operaciones "S1" y "P2" tienen identificadores distintos

        Scenario: 64.4-AO Devolver una bandeja vacía sin confundirla con un fallo
            Given que no existen solicitudes de trabajo
            When consulto la bandeja administrativa de contrataciones
            Then el sistema responde con estado 200
            And la página contiene una colección vacía, no nula, y no tiene cursor siguiente

    Rule: Cada operación expone un resumen acotado y minimizado

        Background:
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"

        Scenario: 64.5-AO Informar partes, rubro, fechas y estados sin exponer datos privados
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor      | prestador        | creada                    | estado   |
                | S1        | ana@example.com | juan@example.com | 2026-09-15T10:00:00-03:00 | accepted |
            And que "S1" tiene una imagen adjunta
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada                    | fecha programada          | duración | estado   |
                | P1        | S1        | 2026-09-16T10:00:00-03:00 | 2026-09-23T10:00:00-03:00 | 120      | accepted |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada                  | estado           | finalización informada    | saldo pagado |
                | O1    | P1        | 2026-09-17T09:00:00-03:00 | awaiting_payment | 2026-09-23T13:00:00-03:00 |              |
            And que la conversación de "S1" tiene mensajes entre "ana@example.com" y "juan@example.com"
            When consulto la bandeja administrativa de contrataciones
            Then la operación "S1" informa al consumidor "Ana Pérez" y al prestador "Juan Gómez" con sus identificadores
            And la operación "S1" informa el rubro "Plomería"
            And la operación "S1" informa los estados de dominio "accepted" de la solicitud, "accepted" de la propuesta y "awaiting_payment" de la orden
            And la operación "S1" informa las siguientes fechas con zona horaria explícita:
                | fecha                  | valor                     |
                | solicitud creada       | 2026-09-15T10:00:00-03:00 |
                | propuesta creada       | 2026-09-16T10:00:00-03:00 |
                | fecha programada       | 2026-09-23T10:00:00-03:00 |
                | orden aceptada         | 2026-09-17T09:00:00-03:00 |
                | finalización informada | 2026-09-23T13:00:00-03:00 |
            And la operación "S1" informa explícitamente como nula la fecha de pago del saldo
            And la respuesta no expone mensajes, extractos del chat, adjuntos, credenciales, biometría ni payloads de pagos
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"
            And no se registra ningún evento de auditoría

    Rule: El responsable de la siguiente acción se deduce de la etapa y nunca se inventa

        Scenario Outline: 64.6-AO Informar el responsable de la siguiente acción para <situación>
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And que existe una operación entre "ana@example.com" y "juan@example.com" con <situación>
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"
            When consulto la bandeja administrativa de contrataciones
            Then la operación entre "ana@example.com" y "juan@example.com" informa que el responsable de la siguiente acción es <responsable>

            Examples:
                | situación                                                        | responsable   |
                | una solicitud pendiente                                          | el prestador  |
                | una solicitud aceptada sin propuestas                            | el prestador  |
                | una propuesta pendiente dentro del límite de pago de la seña     | el consumidor |
                | una propuesta pendiente con el límite de pago de la seña vencido | no deducible  |
                | una orden programada                                             | el prestador  |
                | una orden con finalización informada y saldo pendiente           | el consumidor |
                | una orden pagada                                                 | ninguno       |

    Rule: Las alertas se derivan del estado persistido y de sus fechas sin modificar ese estado

        Background:
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"

        Scenario: 64.7-AO Filtrar solicitudes pendientes de respuesta por más de 24 horas
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor        | prestador         | creada                    | estado   |
                | S1        | ana@example.com   | juan@example.com  | 2026-09-24T12:00:00-03:00 | pending  |
                | S2        | carla@example.com | juan@example.com  | 2026-09-24T11:59:59-03:00 | pending  |
                | S3        | ana@example.com   | pedro@example.com | 2026-09-20T10:00:00-03:00 | accepted |
            When filtro la bandeja por la alerta "request_pending_over_24h"
            Then la bandeja contiene solamente la operación "S2"
            And la operación "S2" informa la alerta "request_pending_over_24h"
            And la operación "S2" informa el estado de dominio "pending" de la solicitud

        Scenario: 64.8-AO Filtrar propuestas pendientes que alcanzaron el límite de pago de la seña
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor        | prestador         | creada                    | estado   |
                | S1        | ana@example.com   | juan@example.com  | 2026-09-20T10:00:00-03:00 | accepted |
                | S2        | carla@example.com | juan@example.com  | 2026-09-20T10:00:00-03:00 | accepted |
                | S3        | ana@example.com   | pedro@example.com | 2026-09-20T10:00:00-03:00 | accepted |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada                    | fecha programada          | duración | estado   |
                | P1        | S1        | 2026-09-21T10:00:00-03:00 | 2026-09-26T12:00:00-03:00 | 60       | pending  |
                | P2        | S2        | 2026-09-21T10:00:00-03:00 | 2026-09-26T12:01:00-03:00 | 60       | pending  |
                | P3        | S3        | 2026-09-21T10:00:00-03:00 | 2026-09-26T10:00:00-03:00 | 60       | accepted |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada                  | estado    | finalización informada | saldo pagado |
                | O3    | P3        | 2026-09-21T12:00:00-03:00 | scheduled |                        |              |
            When filtro la bandeja por la alerta "booking_deadline_passed"
            Then la bandeja contiene solamente la operación "S1"
            And la operación "S1" informa el estado de dominio "pending" de la propuesta

        Scenario: 64.9-AO Filtrar trabajos demorados sin afirmar la inasistencia del prestador
            Given que existen las siguientes órdenes de trabajo con su fecha programada:
                | orden | consumidor        | prestador         | fecha programada          | duración | estado           |
                | O1    | ana@example.com   | juan@example.com  | 2026-09-25T10:00:00-03:00 | 120      | scheduled        |
                | O2    | carla@example.com | juan@example.com  | 2026-09-25T10:00:00-03:00 | 119      | scheduled        |
                | O3    | ana@example.com   | pedro@example.com | 2026-09-25T09:00:00-03:00 | 60       | awaiting_payment |
            When filtro la bandeja por la alerta "delayed"
            Then la bandeja contiene solamente la operación de la orden "O2"
            And la operación de la orden "O2" informa la alerta "delayed"
            And la operación de la orden "O2" informa el estado de dominio "scheduled" de la orden

        Scenario: 64.10-AO Filtrar operaciones estancadas por su último avance de negocio y no por sus mensajes
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor          | prestador         | creada                    | estado   |
                | S1        | ana@example.com     | juan@example.com  | 2026-09-20T10:00:00-03:00 | accepted |
                | S2        | carla@example.com   | juan@example.com  | 2026-09-20T10:00:00-03:00 | accepted |
                | S3        | beatriz@example.com | juan@example.com  | 2026-09-10T10:00:00-03:00 | accepted |
                | S4        | ana@example.com     | pedro@example.com | 2026-09-05T10:00:00-03:00 | accepted |
                | S5        | carla@example.com   | pedro@example.com | 2026-08-20T10:00:00-03:00 | accepted |
                | S6        | beatriz@example.com | pedro@example.com | 2026-09-20T10:00:00-03:00 | pending  |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada                    | fecha programada          | duración | estado   |
                | P1        | S1        | 2026-09-22T11:59:00-03:00 | 2026-10-01T10:00:00-03:00 | 60       | pending  |
                | P2        | S2        | 2026-09-22T12:00:00-03:00 | 2026-10-01T10:00:00-03:00 | 60       | pending  |
                | P3        | S3        | 2026-09-11T10:00:00-03:00 | 2026-09-21T10:00:00-03:00 | 60       | accepted |
                | P4        | S4        | 2026-09-06T10:00:00-03:00 | 2026-09-30T10:00:00-03:00 | 60       | accepted |
                | P5        | S5        | 2026-08-21T10:00:00-03:00 | 2026-08-30T10:00:00-03:00 | 60       | accepted |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada                  | estado           | finalización informada    | saldo pagado              |
                | O3    | P3        | 2026-09-12T10:00:00-03:00 | awaiting_payment | 2026-09-22T11:00:00-03:00 |                           |
                | O4    | P4        | 2026-09-07T10:00:00-03:00 | scheduled        |                           |                           |
                | O5    | P5        | 2026-08-22T10:00:00-03:00 | paid             | 2026-08-30T12:00:00-03:00 | 2026-09-01T10:00:00-03:00 |
            And que la conversación de "S1" tiene un mensaje enviado el "2026-09-25T11:00:00-03:00"
            When filtro la bandeja por la alerta "stalled"
            Then la bandeja contiene solamente las operaciones "S1", "S3" y "S6"
            And la operación "S1" informa como último avance de negocio "2026-09-22T11:59:00-03:00"

        Scenario: 64.11-AO Informar la limitación cuando no existe evidencia del último avance de negocio
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor      | prestador        | creada                    | estado   |
                | S1        | ana@example.com | juan@example.com | 2026-09-10T10:00:00-03:00 | accepted |
            When consulto la bandeja administrativa de contrataciones
            Then la operación "S1" informa explícitamente como nulo su último avance de negocio
            And la operación "S1" informa la limitación "request_acceptance_time_unavailable"
            And la operación "S1" no informa la alerta "stalled"

    Rule: El día programado se interpreta en la zona horaria America/Argentina/Buenos_Aires

        @wip
        Scenario: 64.12-AO Filtrar los trabajos programados para un día de Buenos Aires
            Given que la fecha y hora actual del sistema es "2026-09-25T23:30:00-03:00"
            And que existen las siguientes órdenes de trabajo con su fecha programada:
                | orden | consumidor        | prestador         | fecha programada          | duración | estado           |
                | O1    | ana@example.com   | juan@example.com  | 2026-09-25T00:00:00-03:00 | 60       | awaiting_payment |
                | O2    | carla@example.com | juan@example.com  | 2026-09-25T23:59:00-03:00 | 60       | scheduled        |
                | O3    | ana@example.com   | pedro@example.com | 2026-09-26T00:00:00-03:00 | 60       | scheduled        |
                | O4    | carla@example.com | pedro@example.com | 2026-09-24T23:59:00-03:00 | 60       | scheduled        |
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"
            When filtro la bandeja por el día programado "2026-09-25"
            Then la bandeja contiene solamente las operaciones de las órdenes "O1" y "O2"

    Rule: Los filtros de la bandeja se combinan mediante AND

        Background:
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor        | prestador         | creada                    | estado   |
                | S1        | ana@example.com   | juan@example.com  | 2026-09-20T00:00:00-03:00 | accepted |
                | S2        | carla@example.com | juan@example.com  | 2026-09-20T10:00:00-03:00 | accepted |
                | S3        | ana@example.com   | luis@example.com  | 2026-09-20T10:00:00-03:00 | accepted |
                | S4        | ana@example.com   | pedro@example.com | 2026-09-20T10:00:00-03:00 | accepted |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada                    | fecha programada          | duración | estado   |
                | P1        | S1        | 2026-09-20T10:00:00-03:00 | 2026-09-28T10:00:00-03:00 | 60       | pending  |
                | P2        | S2        | 2026-09-20T11:00:00-03:00 | 2026-09-28T10:00:00-03:00 | 60       | pending  |
                | P3        | S3        | 2026-09-20T11:00:00-03:00 | 2026-09-28T10:00:00-03:00 | 60       | pending  |
                | P4        | S4        | 2026-09-20T11:00:00-03:00 | 2026-09-28T10:00:00-03:00 | 60       | pending  |
                | P6        | S1        | 2026-09-21T00:00:00-03:00 | 2026-09-28T10:00:00-03:00 | 60       | pending  |
                | P7        | S1        | 2026-09-20T12:00:00-03:00 | 2026-09-27T10:00:00-03:00 | 60       | accepted |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada                  | estado    | finalización informada | saldo pagado |
                | O7    | P7        | 2026-09-20T13:00:00-03:00 | scheduled |                        |              |
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"

        @wip
        Scenario: 64.13-AO Combinar consumidor, rubro, rango de inicio y etapa
            When filtro la bandeja por el consumidor "ana@example.com", el rubro "Plomería", el inicio desde "2026-09-20T00:00:00-03:00" hasta "2026-09-21T00:00:00-03:00" y la etapa "proposal_pending"
            Then la bandeja contiene solamente las operaciones "S1" y "S3"
            And el inicio del rango es inclusivo y el fin es exclusivo

        @wip
        Scenario: 64.14-AO Filtrar por prestador
            When filtro la bandeja por el prestador "juan@example.com"
            Then la bandeja contiene solamente las operaciones "S1", "S2", "P6" y "P7"

    Rule: La bandeja es paginada con un orden estable

        Background:
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"

        @wip
        Scenario: 64.15-AO Recorrer páginas en orden estable sin multiplicar operaciones por sus relaciones
            Given que existen las siguientes solicitudes de trabajo:
                | solicitud | consumidor          | prestador         | creada                    | estado   |
                | S1        | ana@example.com     | juan@example.com  | 2026-09-24T12:00:00-03:00 | accepted |
                | S2        | carla@example.com   | juan@example.com  | 2026-09-24T11:00:00-03:00 | pending  |
                | S3        | beatriz@example.com | juan@example.com  | 2026-09-24T11:00:00-03:00 | pending  |
                | S4        | ana@example.com     | pedro@example.com | 2026-09-24T11:00:00-03:00 | pending  |
                | S5        | carla@example.com   | pedro@example.com | 2026-09-24T10:00:00-03:00 | pending  |
            And que "S1" tiene una propuesta con dos intentos de pago de la seña y una orden con finalización informada con tres imágenes
            When recorro la bandeja con páginas de 2 operaciones
            Then obtengo 3 páginas y sólo la última no tiene cursor siguiente
            And las páginas contienen las operaciones "S1", "S2", "S3", "S4" y "S5" una sola vez cada una
            And las operaciones se ordenan por inicio descendente y las operaciones "S2", "S3" y "S4" se desempatan por identificador descendente

        @wip
        Scenario Outline: 64.16-AO Aplicar el límite predeterminado y el máximo documentados
            Given que existen 101 solicitudes de trabajo pendientes sintéticas entre pares distintos de consumidor y prestador
            When consulto la bandeja administrativa de contrataciones <configuración del límite>
            Then la página contiene <cantidad> operaciones y entrega un cursor siguiente

            Examples:
                | configuración del límite | cantidad |
                | sin indicar límite       | 20       |
                | con límite 100           | 100      |

    Rule: La autenticación, el permiso y los parámetros restringen la consulta

        Scenario Outline: 64.17-AO Rechazar la consulta sin autenticación válida
            Given que existe una solicitud de trabajo pendiente para el prestador "juan@example.com"
            And que <estado de autenticación>
            When intento consultar la bandeja administrativa de contrataciones
            Then el sistema responde con estado 401
            And la respuesta no contiene operaciones

            Examples:
                | estado de autenticación        |
                | no envío un token Bearer       |
                | envío un token Bearer inválido |

        Scenario: 64.18-AO Rechazar a un administrador sin el permiso de operaciones
            Given que existe una solicitud de trabajo pendiente para el prestador "juan@example.com"
            And que estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_audit"
            When intento consultar la bandeja administrativa de contrataciones
            Then el sistema responde con estado 403
            And la respuesta no contiene operaciones

        @wip
        Scenario Outline: 64.19-AO Rechazar filtros, rangos, límites y cursores inválidos
            Given que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"
            When consulto la bandeja administrativa de contrataciones con los parámetros "<parámetros>"
            Then el sistema responde con estado 400
            And la respuesta no contiene operaciones

            Examples:
                | parámetros                                                                  |
                | consumer_id=0                                                               |
                | provider_id=abc                                                             |
                | category_id=0                                                               |
                | stage=expired                                                               |
                | alert=scheduled_today                                                       |
                | scheduled_date=2026-09-31                                                   |
                | started_from=no-es-fecha                                                    |
                | started_to=2026-09-20T00:00:00                                              |
                | started_from=2026-09-21T00:00:00-03:00&started_to=2026-09-20T00:00:00-03:00 |
                | limit=0                                                                     |
                | limit=101                                                                   |
                | cursor=no-es-un-cursor                                                      |
