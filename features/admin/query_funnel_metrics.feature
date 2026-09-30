Feature: Consultar métricas administrativas del embudo de contrataciones
    Como responsable de operaciones y producto de LoResuelvo
    quiero consultar la conversión y las demoras del recorrido de contratación
    para identificar en qué etapas las solicitudes avanzan o quedan pendientes

    Background:
        Given que existen los rubros "Plomería" y "Electricidad"
        And que existen los siguientes consumidores registrados:
            | correo              | nombre  | apellido |
            | ana@example.com     | Ana     | Pérez    |
            | beatriz@example.com | Beatriz | Suárez   |
            | carla@example.com   | Carla   | Gómez    |
        And que existen prestadores registrados en los rubros indicados:
            | correo            | nombre | apellido | rubro        |
            | juan@example.com  | Juan   | López    | Plomería     |
            | luis@example.com  | Luis   | Díaz     | Electricidad |
            | pedro@example.com | Pedro  | Gómez    | Plomería     |
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "Martínez"

    Rule: El rango efectivo, la zona horaria y el rubro describen la cohorte consultada
        Background:
            Given que el reloj del sistema indica "2026-09-30T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_metrics"

        @wip
        Scenario: 67.1-AM Aplicar los últimos treinta días e informar los metadatos del período
            Given que no existen evaluaciones profesionales ni solicitudes manuales en el período predeterminado
            When consulto el embudo administrativo sin filtros
            Then el sistema responde con estado 200
            And el período efectivo comienza exactamente treinta días antes de "2026-09-30T12:00:00-03:00" y termina en ese instante
            And la respuesta informa la zona horaria "America/Argentina/Buenos_Aires" y el campo "observed_at" con el instante "2026-09-30T12:00:00-03:00"
            And ambos recorridos informan conteos cero, porcentajes sin denominador como nulos y medias sin observaciones como nulas
            And no se inventa actividad ni se aplica un umbral mínimo de muestras

        @wip
        Scenario: 67.2-AM Seleccionar orígenes dentro de un período personalizado semiabierto
            Given que existen evaluaciones profesionales creadas en "2026-09-01T00:00:00-03:00" y "2026-09-03T00:00:00-03:00"
            And que existe una solicitud manual sin evaluación de origen creada en "2026-09-02T12:00:00-03:00"
            When consulto el embudo desde "2026-09-01T00:00:00-03:00" hasta "2026-09-03T00:00:00-03:00"
            Then el sistema responde con estado 200
            And el período efectivo incluye el origen creado exactamente en "2026-09-01T00:00:00-03:00" y excluye los creados exactamente al final
            And el recorrido IA cuenta una evaluación profesional y el recorrido manual una solicitud

        @wip
        Scenario Outline: 67.3-AM Filtrar cada cohorte por el rubro que le corresponde
            Given que existe una evaluación profesional de "ana@example.com" del rubro "Plomería" en el período
            And que existe una solicitud manual de "beatriz@example.com" dirigida a un prestador cuyo rubro actual es "Electricidad" en el período
            When consulto el embudo para el rubro "<rubro>"
            Then el sistema responde con estado 200
            And el recorrido IA cuenta <conteo_ia> y selecciona por el rubro persistido en la evaluación de origen
            And el recorrido manual cuenta <conteo_manual> y selecciona por el rubro actual del prestador destinatario, sin presentarlo como histórico

            Examples:
                | rubro        | conteo_ia | conteo_manual |
                | Plomería     | una       | cero          |
                | Electricidad | cero      | una           |

        @wip
        Scenario: 67.4-AM Responder sin actividad con rubro desconocido
            Given que no existe actividad asociada al rubro 2147483000
            When consulto el embudo con el rubro con ID 2147483000
            Then el sistema responde con estado 200
            And ambos recorridos contienen conteos cero, porcentajes sin denominador nulos y medias nulas
            And no se devuelve un error de validación por no encontrar ese rubro

    Rule: Las cohortes IA y manual conservan sus propios orígenes e identidades
        Background:
            Given que el reloj del sistema indica "2026-09-30T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_metrics"

        @wip
        Scenario Outline: 67.5-AM Contar cada versión profesional persistida sin sustituir su evaluación de origen
            Given que la conversación asistida de "ana@example.com" tiene las siguientes evaluaciones persistidas en el período:
                | evaluación | versión | resultado             | rubro        | creada                    |
                | A1         | 1       | professional_required | Plomería     | 2026-09-10T10:00:00-03:00 |
                | A2         | 2       | professional_required | Electricidad | 2026-09-11T10:00:00-03:00 |
            And que "A2" es la evaluación actual de la conversación
            And que la solicitud "S1" se originó explícitamente en "A1" aunque "A2" sea la evaluación actual
            When consulto el embudo <consulta>
            Then cada versión profesional persistida se cuenta como unidad independiente
            And el recorrido IA informa <evaluaciones> evaluaciones profesionales y <con_solicitud> con solicitud
            And ninguna etapa cuenta mensajes como unidades ni combina las versiones persistidas

            Examples:
                | consulta                     | evaluaciones | con_solicitud |
                | sin filtros                  | 2            | 1             |
                | para el rubro "Plomería"     | 1            | 1             |
                | para el rubro "Electricidad" | 1            | 0             |

        @wip
        Scenario: 67.6-AM No crear otra unidad por resultado unchanged y excluir evaluaciones ajenas a la cohorte
            Given que existe una evaluación persistida "A1" con resultado "professional_required" en el período
            And que una respuesta posterior del asistente tiene acción "unchanged" y reutiliza el ID de "A1" sin persistir una nueva versión
            And que existen evaluaciones del período con resultados "self_service" y "collecting_information"
            When consulto el embudo sin filtros
            Then el recorrido IA cuenta una sola unidad para "A1"
            And los resultados "self_service" y "collecting_information" no se incluyen en la cohorte de contratación
            And los mensajes y la respuesta "unchanged" no incrementan el conteo de evaluaciones

        @wip
        Scenario: 67.7-AM Separar solicitudes manuales de las solicitudes originadas en IA
            Given que existe una solicitud manual de "ana@example.com" con "juan@example.com" cuyo source_assessment_id está ausente
            And que existe otra solicitud de "beatriz@example.com" con "pedro@example.com" originada en una evaluación profesional persistida
            When consulto el embudo sin filtros
            Then la cohorte manual contiene únicamente la solicitud sin evaluación de origen
            And la cohorte IA cuenta la evaluación profesional y solo considera la solicitud vinculada a su ID de origen
            And las dos cohortes mantienen denominadores independientes y no se suman entre sí

    Rule: Las etapas cuentan unidades distintas y siguen una cadena de relaciones persistidas
        Background:
            Given que el reloj del sistema indica "2026-09-30T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_metrics"

        @wip
        Scenario: 67.8-AM No multiplicar las etapas de una cohorte IA con varias ramas e intentos de pago
            Given que existen tres evaluaciones profesionales del período: "A1", "A2" y "A3"
            And que "A1" se vinculó a dos solicitudes distintas para prestadores distintos y ambas tienen propuestas en sus propias conversaciones
            And que la primera solicitud de "A1" tiene dos propuestas, cada una con su propia orden, y que la segunda tiene una propuesta y su orden
            And que "A2" tiene una solicitud, una propuesta y una orden; "A3" no tiene solicitud
            And que una orden de "A1" tiene reporte persistido, está pagada y tiene una reseña; la orden de "A2" tiene reporte pero aún no tiene paid_on
            And que una orden de "A1" tiene varios intentos de pago y transacciones, incluidos intentos no aprobados
            When consulto el embudo sin filtros
            Then el recorrido IA informa estos conteos y conversiones consecutivas por cohorte, sin multiplicar una unidad por sus relaciones 1:N:
                | etapa                       | unidades | conversión |
                | evaluación profesional      | 3        |            |
                | con solicitud               | 2        | 66.67 %    |
                | con propuesta               | 2        | 100.00 %   |
                | con contratación confirmada | 2        | 100.00 %   |
                | con finalización informada  | 2        | 100.00 %   |
                | con pago completo           | 1        | 50.00 %    |
                | con reseña                  | 1        | 100.00 %   |
            And las órdenes vinculadas y sus hitos persistidos determinan las etapas; los intentos o transacciones de pago no multiplican etapas ni representan por sí solos el pago completo
            And la conversión global desde el origen hasta "con finalización informada" es 66.67 por ciento aunque pago completo y reseña tengan otros conteos

        @wip
        Scenario: 67.9-AM Contar las etapas y conversiones de la cohorte manual por separado
            Given que existen tres solicitudes manuales del período: "S1", "S2" y "S3"
            And que "S1" tiene dos propuestas en su conversación de trabajo, "S2" una propuesta y "S3" una propuesta cuyo intento de pago de seña está en checkout_ready y no tiene orden
            And que las dos propuestas de "S1" y la de "S2" tienen órdenes aceptadas distintas "O1", "O1b" y "O2"
            And "O1" y "O2" tienen reporte de finalización; solo "O1" tiene "paid_on" y una reseña asociada, mientras "O1b" no tiene reporte
            When consulto el embudo sin filtros
            Then el recorrido manual informa estos conteos y conversiones consecutivas:
                | etapa                       | unidades | conversión |
                | solicitud                   | 3        |            |
                | con propuesta               | 3        | 100.00 %   |
                | con contratación confirmada | 2        | 66.67 %    |
                | con finalización informada  | 2        | 100.00 %   |
                | con pago completo           | 1        | 50.00 %    |
                | con reseña                  | 1        | 100.00 %   |
            And la conversión global desde solicitud hasta finalización informada es 66.67 por ciento
            And las dos propuestas de "S1" no multiplican el conteo de solicitudes ni de etapas
            And el checkout_ready de la propuesta de "S3" no cuenta como contratación confirmada sin una orden vinculada

        @wip
        Scenario: 67.10-AM No combinar hitos de órdenes distintas para completar una misma rama
            Given que una evaluación profesional "A1" originó solicitudes para dos prestadores en conversaciones de trabajo distintas
            And que la orden "O1" de la primera rama tiene un reporte persistido y no tiene "paid_on"
            And que la orden "O2" de la segunda rama tiene "paid_on" persistido pero no tiene reporte de finalización
            And que estos datos incompatibles con el flujo normal son un fixture histórico persistido explícitamente, no una secuencia creada por el flujo de aceptación
            When consulto el embudo sin filtros
            Then el recorrido IA cuenta una unidad con finalización informada por el reporte de "O1"
            And el recorrido IA no cuenta una unidad con pago completo porque ningún mismo recorrido de orden tiene ambos hitos de finalización y pago
            And el reporte de "O1" no se combina con el pago de "O2" para fabricar una cadena completa

        @wip
        Scenario: 67.11-AM Conservar avances ocurridos después del período de origen
            Given que la evaluación profesional "A1" fue creada exactamente al inicio del período
            And que su solicitud, propuesta, orden aceptada, reporte de finalización, pago completo y reseña asociada a esa misma orden pagada ocurrieron después del final del período y antes del instante de observación
            When consulto el embudo desde "2026-09-01T00:00:00-03:00" hasta "2026-09-02T00:00:00-03:00"
            Then "A1" pertenece a la cohorte por su creación dentro del período
            And el recorrido IA informa estos conteos y conversiones consecutivas:
                | etapa                       | unidades | conversión |
                | evaluación profesional      | 1        |            |
                | con solicitud               | 1        | 100.00 %   |
                | con propuesta               | 1        | 100.00 %   |
                | con contratación confirmada | 1        | 100.00 %   |
                | con finalización informada  | 1        | 100.00 %   |
                | con pago completo           | 1        | 100.00 %   |
                | con reseña                  | 1        | 100.00 %   |
            And la conversión global desde evaluación profesional hasta finalización informada es 100.00 por ciento
            And el recorrido manual informa cero unidades y conversiones nulas por tener denominador inicial cero
            And el período no vuelve a filtrar las fechas de solicitudes, propuestas, órdenes, reportes, pagos ni reseñas
            And las etapas anteriores siguen contándose cuando la unidad avanza a otra etapa

        @wip
        Scenario: 67.12-AM Distinguir denominadores nulos de conversiones con cero avance
            Given que existen tres evaluaciones profesionales en el período y ninguna tiene una solicitud
            And que no existen solicitudes manuales en el período
            When consulto el embudo sin filtros
            Then la conversión IA de evaluación profesional a solicitud es 0.00 por ciento con denominador tres
            And las conversiones posteriores con denominador cero son nulas y conservan sus conteos cero
            And la conversión global IA hasta finalización informada es 0.00 por ciento con denominador tres
            And las conversiones manuales son nulas porque su cohorte inicial no tiene unidades

    Rule: Las demoras representan intervalos válidos entre recursos del mismo recorrido
        Background:
            Given que el reloj del sistema indica "2026-09-30T12:00:00-03:00"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_metrics"

        @wip
        Scenario: 67.13-AM Medir muestras por solicitud y por orden sin incluir espera pendiente
            Given que existen estas solicitudes manuales del período:
                | solicitud | consumidor          | prestador         | creada                   |
                | S1        | ana@example.com     | juan@example.com  | 2026-09-10T10:00:00.000Z |
                | S2        | beatriz@example.com | pedro@example.com | 2026-09-10T10:10:00.000Z |
                | S3        | carla@example.com   | juan@example.com  | 2026-09-10T11:00:00.000Z |
            And que las propuestas están vinculadas a sus propias solicitudes y tienen estas fechas:
                | propuesta | solicitud | creada                   | programada               |
                | P1        | S1        | 2026-09-10T10:00:01.000Z | 2026-09-12T10:00:00.000Z |
                | P2        | S1        | 2026-09-10T10:00:05.000Z | 2026-09-13T10:00:00.000Z |
                | P3        | S2        | 2026-09-10T10:11:05.670Z | 2026-09-12T10:11:15.000Z |
            And las órdenes aceptadas, reportes y pagos están vinculados a sus propias propuestas:
                | orden | propuesta | aceptada                 | reporte                  | paid_on                  |
                | O1    | P1        | 2026-09-10T10:00:13.500Z | 2026-09-12T10:00:23.500Z | 2026-09-12T10:01:03.500Z |
                | O2    | P2        | 2026-09-10T10:00:25.000Z |                          |                          |
                | O3    | P3        | 2026-09-10T10:11:15.670Z | 2026-09-12T10:11:45.670Z |                          |
            And "S3" no tiene propuesta y "O2" no tiene reporte
            When consulto el embudo desde "2026-09-10T00:00:00Z" hasta "2026-09-11T00:00:00Z"
            Then la demora solicitud→primera propuesta tiene dos observaciones y media 33.34 segundos, redondeada a dos decimales por mitades hacia arriba
            And la demora propuesta→contratación confirmada tiene tres observaciones de las órdenes "O1", "O2" y "O3" y media 14.17 segundos
            And la demora contratación→finalización tiene dos observaciones y media 172820.00 segundos usando los reportes de "O1" y "O3"
            And la demora finalización→pago tiene una observación y media 40.00 segundos usando el pago de "O1"
            And las demoras usan solo hitos persistidos y cronológicamente válidos de la misma orden
            And los reportes y pagos posteriores al final del período se incluyen por pertenecer a la cohorte seleccionada por la fecha de sus solicitudes
            And la solicitud y la orden sin reporte no aportan como duración el tiempo que llevan esperando

        @wip
        Scenario: 67.14-AM Excluir solo el intervalo con cronología inválida sin perder otras muestras
            Given que la solicitud "S1" fue creada a las "2026-09-10T10:00:00Z" y sus propuestas persistidas se crearon a las "2026-09-10T09:59:00Z" y "2026-09-10T10:02:00Z"
            And que la orden histórica de la primera propuesta fue aceptada a las "2026-09-10T10:03:00Z" y tiene un reporte anterior a su aceptación
            And que la orden histórica de la primera propuesta no tiene "paid_on"
            And que la solicitud "S2" fue creada a las "2026-09-10T10:30:00Z", su propuesta a las "2026-09-10T10:31:00Z", su orden aceptada a las "2026-09-10T10:32:00Z", programada para "2026-09-12T10:31:00Z", reportada a las "2026-09-12T10:32:00Z" y pagada a las "2026-09-12T10:33:00Z"
            And que las fechas anómalas de "S1" provienen de un fixture histórico persistido explícitamente, no del flujo normal de contratación
            When consulto el embudo desde "2026-09-10T00:00:00Z" hasta "2026-09-11T00:00:00Z"
            Then la demora solicitud→primera propuesta tiene una observación de "S2" y media 60.00 segundos
            And no se sustituye la primera propuesta inválida por una posterior
            And la demora propuesta→contratación confirmada conserva dos observaciones válidas y una media de 150.00 segundos
            And el intervalo contratación→finalización inválido de la primera orden se excluye sin descartar la observación válida de "S2" de 172800.00 segundos
            And la demora finalización→pago conserva una observación válida de "S2" y media 60.00 segundos
            And cada media informa su cantidad de observaciones válidas

        @wip
        Scenario: 67.15-AM Informar medias nulas o cero según existan observaciones válidas
            Given que la única solicitud manual del período fue creada a las "2026-09-10T10:00:00Z" y su primera propuesta persistida se creó en ese mismo instante, sin orden
            When consulto el embudo desde "2026-09-10T00:00:00Z" hasta "2026-09-11T00:00:00Z"
            Then solicitud→primera propuesta informa una observación válida y una media de cero segundos
            And las otras medias informan cero observaciones y valor nulo

        @wip
        Scenario: 67.16-AM Aceptar un período de exactamente 365 días que termina al observar
            Given que no hay actividad dentro del período
            When consulto el embudo desde "2025-09-30T15:00:00Z" hasta "2026-09-30T15:00:00Z"
            Then el sistema responde con estado 200
            And el período efectivo tiene exactamente 365 días y su final coincide con el instante de observación

    Rule: La consulta requiere permiso, rango válido y lectura disponible, y solo devuelve agregados
        Background:
            Given que el reloj del sistema indica "2026-09-30T12:00:00-03:00"

        @wip
        Scenario Outline: 67.17-AM Rechazar rangos incompletos o fuera de los límites permitidos
            Given que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_metrics"
            When consulto el embudo con los parámetros "<parámetros>"
            Then el sistema responde con estado 400 y no devuelve métricas
            And la respuesta incluye "Cache-Control: private, no-store"

            Examples:
                | parámetros                                                                  |
                | from=2026-09-01T00:00:00Z                                                   |
                | to=2026-09-02T00:00:00Z                                                     |
                | from=fecha&to=2026-09-02T00:00:00Z                                          |
                | from=2026-09-01T00:00:00&to=2026-09-02T00:00:00                             |
                | from=2026-09-02T00:00:00Z&to=2026-09-02T00:00:00Z                           |
                | from=2026-09-03T00:00:00Z&to=2026-09-02T00:00:00Z                           |
                | from=2025-09-30T14:59:59Z&to=2026-09-30T15:00:00Z                           |
                | from=2026-09-01T00:00:00Z&to=2026-09-30T15:00:01Z                           |
                | category_id=0                                                               |
                | category_id=-1                                                              |
                | category_id=no-numérico                                                     |
                | category_id=                                                                |
                | from=2026-09-01T00:00:00Z&from=2026-09-02T00:00:00Z&to=2026-09-03T00:00:00Z |
                | diagnostico=true                                                            |

        @wip
        Scenario: 67.18-AM Exigir el permiso de métricas administrativas
            Given que estoy autenticado con un JWT válido de administrador sin el permiso "read:admin_metrics"
            And que el JWT incluye otro permiso administrativo
            When consulto el embudo administrativo
            Then el sistema responde con estado 403 y no devuelve métricas
            And la respuesta incluye "Cache-Control: private, no-store"

        @wip
        Scenario: 67.19-AM No entregar datos si falla la lectura coherente de métricas
            Given que existe actividad válida dentro del período
            And que el reader administrativo falla al obtener la instantánea coherente de lectura
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_metrics"
            When consulto el embudo administrativo
            Then el sistema responde con estado 500
            And el error no se presenta como una respuesta vacía ni como conteos cero
            And no se entrega un agregado parcial
            And la respuesta incluye "Cache-Control: private, no-store"

        @wip
        Scenario: 67.20-AM Proteger el acceso y limitar la respuesta a agregados sin auditoría ni efectos de negocio
            Given que existen evaluaciones, solicitudes, propuestas y órdenes con actividad en el período
            And que no existe un evento administrativo de auditoría para la consulta
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_metrics"
            When consulto el embudo administrativo
            Then el sistema responde con estado 200
            And la respuesta incluye "Cache-Control: private, no-store"
            And la respuesta contiene solo agregados y metadatos de cálculo, sin filas individuales, datos personales, conversaciones ni payloads financieros
            And no se registra auditoría administrativa persistente ni se modifica el estado de negocio
            And la consulta no depende de servicios externos

        @wip
        Scenario Outline: 67.21-AM Rechazar identidades ausentes o inválidas
            Given que <identidad>
            When consulto el embudo administrativo
            Then el sistema responde con estado <estado> y no devuelve métricas
            And la respuesta incluye "Cache-Control: private, no-store"

            Examples:
                | identidad              | estado |
                | no inicié sesión       | 401    |
                | mi sesión no es válida | 401    |
