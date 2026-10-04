@wip
Feature: Consultar la conversión de mis propuestas de servicio
    Como prestador de LoResuelvo
    quiero conocer qué proporción de mis propuestas se convirtió en contrataciones, finalizaciones informadas y pagos completos
    para evaluar los resultados de mis presupuestos siguiendo las mismas propuestas a través del flujo de servicio

    Background:
        Given que existen los consumidores "ana@example.com", "carla@example.com", "beatriz@example.com", "diego@example.com" y "elena@example.com"
        And que existen los prestadores "juan@example.com" y "pedro@example.com"
        And que hoy es 29 de septiembre de 2026 a las 12:00 en Buenos Aires
        And que estoy autenticado como prestador "juan@example.com"

    Rule: El embudo sigue la cohorte de propuestas creadas en el período, no los hitos ocurridos en él

        Scenario: 73.1-CV Contar las etapas alcanzadas por las propuestas de mi cohorte
            Given que tengo estas propuestas:
                | propuesta | cliente             | creada el       | estado actual | contratación confirmada | finalización informada | pago completo    |
                | P1        | ana@example.com     | 1 de septiembre | aceptada      | 10 de septiembre        | 12 de septiembre       | 15 de septiembre |
                | P2        | carla@example.com   | 2 de septiembre | aceptada      | 3 de septiembre         |                        |                  |
                | P3        | beatriz@example.com | 3 de septiembre | rechazada     |                         |                        |                  |
                | P4        | diego@example.com   | 4 de septiembre | pendiente     |                         |                        |                  |
            And que tengo cuatro propuestas creadas en agosto, contratadas entre el 2 y el 4 de septiembre
            And que "pedro@example.com" tiene una propuesta creada el 2 de septiembre, contratada el 3, finalizada el 4 y pagada el 5 de septiembre
            When consulto la conversión desde el 1 hasta antes del 5 de septiembre de 2026
            Then veo estas etapas de mi cohorte:
                | etapa                      | propuestas |
                | emitidas                   | 4          |
                | contratadas                | 2          |
                | con finalización informada | 1          |
                | con pago completo          | 1          |
            And P1 cuenta en las cuatro etapas aunque su trabajo ya esté pagado
            And la contratación posterior de P1, aunque ocurrió después del período, cuenta en su cohorte
            And las cuatro propuestas anteriores no cuentan aunque sus contrataciones ocurrieron dentro del período
            And la conversión de mi cohorte es 50,00 %, no la tasa que resultaría de dividir los hitos del período por las propuestas emitidas
            And los resultados corresponden exclusivamente a mis propuestas, no a las de "pedro@example.com"
            And no veo comparaciones con otra cohorte, variaciones entre períodos ni espacios reservados para esos resultados

        Scenario Outline: 73.2-CV Respetar los límites inclusivo y exclusivo de creación
            Given que tengo una propuesta creada <momento>
            When consulto la conversión desde el 1 hasta antes del 5 de septiembre de 2026
            Then <resultado>

            Examples:
                | momento                                | resultado                 |
                | el 31 de agosto a las 23:59 de 2026    | veo 0 propuestas emitidas |
                | el 1 de septiembre a las 00:00 de 2026 | veo 1 propuesta emitida   |
                | el 4 de septiembre a las 23:59 de 2026 | veo 1 propuesta emitida   |
                | el 5 de septiembre a las 00:00 de 2026 | veo 0 propuestas emitidas |

        Scenario: 73.3-CV Contar propuestas distintas aunque compartan conversación o tengan relaciones múltiples
            Given que tengo dos propuestas distintas para "ana@example.com" en una misma conversación, creadas el 2 y el 3 de septiembre
            And que ambas propuestas fueron contratadas, tienen finalización informada y pago completo antes del 28 de septiembre
            And que el reporte de finalización de la orden vinculada a una propuesta tiene tres fotos
            And que la orden de la otra propuesta tiene varios intentos de pago, uno rechazado y otro aprobado
            When consulto la conversión desde el 1 hasta antes del 29 de septiembre de 2026
            Then veo 2 propuestas emitidas, 2 contratadas, 2 con finalización informada y 2 con pago completo

        Scenario: 73.4-CV Mostrar las tasas de avance con sus denominadores explícitos
            Given que tengo estas propuestas creadas antes del 29 de septiembre de 2026:
                | propuesta | creada el       | contratación confirmada | finalización informada | pago completo   |
                | P1        | 1 de septiembre | 2 de septiembre         | 3 de septiembre        | 4 de septiembre |
                | P2        | 2 de septiembre | 3 de septiembre         | 4 de septiembre        |                 |
                | P3        | 3 de septiembre | 4 de septiembre         |                        |                 |
                | P4        | 4 de septiembre | 5 de septiembre         |                        |                 |
                | P5        | 5 de septiembre |                         |                        |                 |
                | P6        | 6 de septiembre |                         |                        |                 |
                | P7        | 7 de septiembre |                         |                        |                 |
                | P8        | 8 de septiembre |                         |                        |                 |
            When consulto la conversión desde el 1 hasta antes del 29 de septiembre de 2026
            Then veo estas tasas, calculadas con los conteos indicados:
                | tasa                                 | numerador | denominador                             | resultado |
                | contratación sobre la cohorte        | 4         | 8 propuestas emitidas                   | 50,00 %   |
                | contratación sobre la etapa anterior | 4         | 8 propuestas emitidas                   | 50,00 %   |
                | finalización sobre la cohorte        | 2         | 8 propuestas emitidas                   | 25,00 %   |
                | finalización sobre la etapa anterior | 2         | 4 propuestas contratadas                | 50,00 %   |
                | pago sobre la cohorte                | 1         | 8 propuestas emitidas                   | 12,50 %   |
                | pago sobre la etapa anterior         | 1         | 2 propuestas con finalización informada | 50,00 %   |

        Scenario: 73.5-CV Redondear una tasa cuando queda justo a mitad de un centésimo
            Given que tengo 32 propuestas creadas entre el 1 y el 28 de septiembre, una creada el 1 fue contratada el 10 y las otras 31 no tienen orden
            When consulto la conversión desde el 1 hasta antes del 29 de septiembre de 2026
            Then la tasa de contratación sobre la cohorte es 3,13 %, calculada como 1 propuesta contratada sobre 32 emitidas

        Scenario: 73.6-CV Informar porcentajes sin valor o cero según el denominador
            Given que tengo 3 propuestas creadas entre el 1 y el 3 de septiembre y ninguna fue contratada
            When consulto la conversión desde el 1 hasta antes del 4 de septiembre de 2026
            Then veo 0 propuestas contratadas, con finalización informada y con pago completo
            And las tres tasas sobre la cohorte son 0,00 %
            And la contratación respecto de las emitidas es 0,00 %
            And la finalización y el pago respecto de su etapa anterior se muestran sin valor porque esos denominadores son cero

        Scenario: 73.7-CV Distinguir la falta de contratación de un rechazo
            Given que tengo tres propuestas creadas entre el 1 y el 3 de septiembre:
                | propuesta | creada el       | estado actual | contratación confirmada |
                | P1        | 1 de septiembre | aceptada      | 2 de septiembre         |
                | P2        | 2 de septiembre | pendiente     |                         |
                | P3        | 3 de septiembre | rechazada     |                         |
            When consulto la conversión desde el 1 hasta antes del 4 de septiembre de 2026
            Then veo 2 propuestas sin contratación observada, calculadas como 3 emitidas menos 1 contratada
            And las propuestas conservan sus estados aceptada, pendiente y rechazada
            And P2 no se presenta como rechazada por no tener una contratación observada

    Rule: La aceptación de solicitudes es un indicador independiente de la cohorte de propuestas

        Scenario: 73.8-CV Contar mis solicitudes dentro del período según su estado actual
            Given que tengo estas solicitudes:
                | prestador         | cliente             | recibida el     | estado actual |
                | juan@example.com  | ana@example.com     | 2 de septiembre | aceptada      |
                | juan@example.com  | carla@example.com   | 3 de septiembre | aceptada      |
                | juan@example.com  | beatriz@example.com | 4 de septiembre | pendiente     |
                | juan@example.com  | diego@example.com   | 31 de agosto    | pendiente     |
                | juan@example.com  | elena@example.com   | 5 de septiembre | aceptada      |
                | pedro@example.com | ana@example.com     | 2 de septiembre | aceptada      |
                | pedro@example.com | carla@example.com   | 3 de septiembre | pendiente     |
            And que la solicitud que recibí de "ana@example.com" fue aceptada después del final del período
            And que acepté la solicitud de "carla@example.com" el 3 de septiembre antes de crear cuatro propuestas en nuestra conversación el 4 de septiembre; una fue contratada el 10 y las otras tres no tienen orden
            When consulto la conversión desde el 1 hasta antes del 5 de septiembre de 2026
            Then veo 3 solicitudes recibidas, 2 aceptadas y 1 pendiente
            And la aceptación de solicitudes es 66,67 %, calculada como 2 aceptadas sobre 3 recibidas
            And la solicitud aceptada después del período cuenta según su estado actual
            And mi conversión de propuestas es 25,00 %, calculada como 1 contratada sobre 4 emitidas, independiente del 66,67 % de solicitudes aceptadas
            And no se cuentan las solicitudes recibidas antes del inicio, en el final del período ni para "pedro@example.com"

        Scenario: 73.9-CV Informar solicitudes aunque no tenga propuestas en la cohorte
            Given que no tengo propuestas creadas entre el 1 y el 4 de septiembre
            And que recibí una solicitud pendiente el 3 de septiembre
            When consulto la conversión desde el 1 hasta antes del 5 de septiembre de 2026
            Then veo cero propuestas en todas las etapas y porcentajes sin valor por denominador cero
            And veo 1 solicitud recibida, 0 aceptadas, 1 pendiente y 0,00 % de aceptación

        Scenario: 73.10-CV Mostrar resultados vacíos cuando no tengo propuestas ni solicitudes
            Given que no tengo propuestas ni solicitudes recibidas entre el 30 de agosto y el 29 de septiembre de 2026
            When consulto la conversión sin elegir un período
            Then veo cero propuestas y solicitudes en todos los conteos
            And todos los porcentajes se muestran sin valor porque sus denominadores son cero

    Rule: El período predeterminado y los períodos elegidos respetan los límites admitidos

        Scenario: 73.11-CV Usar los últimos treinta días y mostrar el período observado
            Given que tengo propuestas creadas antes del 30 de agosto a las 12:00, exactamente a esa hora, antes del 29 de septiembre a las 12:00 y exactamente al final del período
            When consulto la conversión sin elegir un período
            Then veo 2 propuestas emitidas en el período desde el 30 de agosto de 2026 a las 12:00 hasta el 29 de septiembre de 2026 a las 12:00 en Buenos Aires
            And veo que los avances se observaron el 29 de septiembre de 2026 a las 12:00

        Scenario Outline: 73.12-CV Rechazar rangos incompletos o fuera de los límites permitidos
            When intento consultar la conversión <elección>
            Then se me informa que el período no es válido y no se muestran resultados

            Examples:
                | elección                                  |
                | sin indicar el comienzo                   |
                | sin indicar el final                      |
                | con el mismo comienzo y final             |
                | con un final anterior al comienzo         |
                | por más de 365 días                       |
                | con un final posterior al momento actual  |
                | con una fecha sin indicar su zona horaria |
                | con una fecha en un formato no admitido   |

    Rule: Solo consulto mis resultados con una identidad válida y una lectura completa

        Scenario Outline: 73.13-CV Impedir la consulta sin una identidad de prestador válida
            Given que <identidad>
            When intento consultar la conversión
            Then <resultado> y no se muestran resultados

            Examples:
                | identidad                                                   | resultado                                                       |
                | no inicié sesión                                            | se rechaza la consulta porque no inicié sesión                  |
                | mi sesión no es válida                                      | se rechaza la consulta porque mi sesión no es válida            |
                | estoy autenticado como consumidor "ana@example.com"         | se rechaza la consulta porque no soy prestador                  |
                | inicié sesión con una identidad que no figura en LoResuelvo | se rechaza la consulta porque no tengo una cuenta en LoResuelvo |

        Scenario: 73.14-CV Mantener privados los resultados y no modificar registros al consultar
            Given que tengo una propuesta pendiente y una solicitud aceptada, y en otra conversación una propuesta aceptada con su orden pagada y un mensaje
            And que todos esos registros se crearon entre el 1 y el 28 de septiembre de 2026, y no tengo una cuenta de cobros conectada
            When consulto la conversión desde el 1 hasta antes del 29 de septiembre de 2026
            Then veo únicamente indicadores agregados propios, sin conversaciones, datos de clientes ni información financiera
            And los resultados son privados y no se almacenan en caché
            And las propuestas, las solicitudes, la orden y el mensaje conservan su información
