Feature: Consultar los resultados de mi actividad como prestador
    Como prestador de LoResuelvo
    quiero ver mis contrataciones confirmadas, finalizaciones informadas, cobros y pendientes
    para entender cómo marcha mi actividad y qué debo atender ahora

    Background:
        Given que existen los consumidores "ana@example.com", "carla@example.com", "beatriz@example.com" y "diego@example.com"
        And que existen los prestadores "juan@example.com" y "pedro@example.com"
        And que hoy es 29 de septiembre de 2026 a las 12:00 en Buenos Aires
        And que estoy autenticado como prestador "juan@example.com"

    Rule: Mis resultados reflejan únicamente mis trabajos y la fecha de cada avance

        @wip
        Scenario: 70.1-AP Ver mis contrataciones, finalizaciones informadas y cobros según cuándo ocurrió cada uno
            Given que existen estos trabajos con sus acuerdos aceptados:
                | trabajo | cliente           | prestador         | precio     | comisión  | contratación confirmada | finalización informada | cobrado          |
                | O1      | ana@example.com   | juan@example.com  | ARS 100,01 | ARS 10,00 | 2026-09-02 10:00        | 2026-09-04 10:00       | 2026-09-07 10:00 |
                | O2      | carla@example.com | juan@example.com  | ARS 200,02 | ARS 20,00 | 2026-08-31 10:00        | 2026-09-05 10:00       |                  |
                | O3      | ana@example.com   | juan@example.com  | ARS 300,00 | ARS 30,00 | 2026-09-06 10:00        |                        |                  |
                | O4      | ana@example.com   | juan@example.com  | ARS 400,00 | ARS 40,00 | 2026-08-28 10:00        | 2026-08-30 10:00       | 2026-09-08 10:00 |
                | O5      | ana@example.com   | pedro@example.com | ARS 500,00 | ARS 50,00 | 2026-09-03 10:00        | 2026-09-04 10:00       | 2026-09-07 10:00 |
            When consulto mis resultados desde el 1 hasta antes del 9 de septiembre de 2026
            Then veo 2 contrataciones confirmadas, 2 finalizaciones informadas, 2 cobros completos y 2 clientes atendidos
            And veo un cliente nuevo y uno recurrente
            And el valor pactado de los trabajos cuya finalización informé es ARS 300,03 y su promedio es ARS 150,02, sin sumar las comisiones
            And el trabajo cobrado el 8 de septiembre cuenta como cobro del período, pero no como finalización informada en el período
            And no aparecen los resultados de "pedro@example.com"
            And no veo una comparación con un período anterior

        @wip
        Scenario: 70.2-AP Distinguir a un cliente recurrente de otro que me contrata por primera vez
            Given que informé la finalización de un trabajo para "ana@example.com" antes de septiembre de 2026
            And que informé la finalización de un trabajo para "beatriz@example.com" antes de septiembre de 2026
            And que informé la finalización de un trabajo para "ana@example.com", uno para "beatriz@example.com" y dos para "carla@example.com" entre el 1 y el 3 de septiembre de 2026
            And que "pedro@example.com" informó la finalización de un trabajo para "carla@example.com" antes de septiembre, pero yo no había informado ninguno para ella
            When consulto mis resultados desde el 1 hasta antes del 4 de septiembre de 2026
            Then veo 4 finalizaciones informadas y 3 clientes atendidos
            And veo 2 clientes recurrentes y 1 nuevo, sin contar dos veces a quien tuvo más de un trabajo

        @wip
        Scenario: 70.3-AP Contar cada trabajo una sola vez aunque tenga fotos, reseñas e intentos de pago
            Given que durante los primeros nueve días de septiembre de 2026 me contrataron, informé la finalización y cobré dos trabajos de ARS 100,01 cada uno para "ana@example.com"
            And que el primer trabajo tiene tres fotos, una reseña y un intento de pago fallido antes del aprobado
            And que el segundo tiene dos fotos y un intento de pago fallido antes del aprobado
            When consulto mis resultados desde el 1 hasta antes del 10 de septiembre de 2026
            Then veo 2 contrataciones confirmadas, 2 finalizaciones informadas, 2 cobros completos y un cliente atendido
            And veo un valor pactado de ARS 200,02 y un promedio de ARS 100,01

    Rule: Puedo seguir la actividad por días, semanas o meses, incluso donde no hubo movimiento

        @wip
        Scenario: 70.4-AP Ver mis resultados hasta el 3 de septiembre sin incluir cobros del día siguiente
            Given que me contrataron para un trabajo el 1 de septiembre de 2026 a las 00:00 e informé su finalización el 4 de septiembre a las 10:00
            And que me contrataron para otro trabajo el 31 de agosto de 2026, informé su finalización el 2 de septiembre a las 23:59:59 y lo cobré a partir del 4 de septiembre
            And que cobré un tercer trabajo el 4 de septiembre de 2026 a las 00:00, cuya contratación se confirmó y cuya finalización informé antes de septiembre
            When consulto mi evolución diaria desde el 1 hasta antes del 4 de septiembre de 2026
            Then veo exactamente estos días, en orden:
                | día             | contrataciones | finalizaciones informadas | cobros completos |
                | 1 de septiembre | 1              | 0                         | 0                |
                | 2 de septiembre | 0              | 1                         | 0                |
                | 3 de septiembre | 0              | 0                         | 0                |
            And no veo cobros completos en el total de esos tres días

        @wip
        Scenario: 70.5-AP Ver una contratación y una finalización a cada lado del inicio de semana
            Given que me contrataron para un trabajo el domingo 6 de septiembre de 2026 a las 23:30
            And que informé su finalización el martes 8 de septiembre de 2026 a las 00:30
            When consulto mi evolución semanal desde el domingo 6 a las 12:00 hasta el martes 8 a las 12:00
            Then veo una contratación en el tramo desde el domingo 6 a las 12:00 hasta el lunes 7 a las 00:00
            And veo una finalización informada en el tramo desde el lunes 7 a las 00:00 hasta el martes 8 a las 12:00
            And no veo otros tramos en el período elegido

        @wip
        Scenario: 70.6-AP Ver una contratación y una finalización a cada lado del inicio de mes
            Given que me contrataron para un trabajo el 31 de agosto de 2026 a las 23:30
            And que informé su finalización el 2 de septiembre de 2026 a las 00:30
            When consulto mi evolución mensual desde el 31 de agosto a las 12:00 hasta el 2 de septiembre a las 12:00
            Then veo una contratación en el tramo desde el 31 de agosto a las 12:00 hasta el 1 de septiembre a las 00:00
            And veo una finalización informada en el tramo desde el 1 de septiembre a las 00:00 hasta el 2 de septiembre a las 12:00
            And no veo otros tramos en el período elegido

    Rule: Puedo comparar mis resultados con el período inmediatamente anterior

        @wip
        Scenario: 70.7-AP Comparar el crecimiento de mis resultados con los dos días anteriores
            Given que entre el 8 y el 9 de septiembre de 2026 me contrataron, informé la finalización y cobré un trabajo de ARS 100,00 para un cliente
            And que entre el 10 y el 11 de septiembre de 2026 me contrataron, informé la finalización y cobré dos trabajos de ARS 100,00 para dos clientes distintos
            When comparo mis resultados del 10 y 11 de septiembre de 2026 con el período anterior
            Then la comparación corresponde al 8 y 9 de septiembre de 2026
            And veo estas diferencias entre el período elegido y el anterior:
                | resultado                 | actual     | anterior   | diferencia | cambio porcentual |
                | contrataciones            | 2          | 1          | 1          | 100 %             |
                | finalizaciones informadas | 2          | 1          | 1          | 100 %             |
                | cobros completos          | 2          | 1          | 1          | 100 %             |
                | clientes atendidos        | 2          | 1          | 1          | 100 %             |
                | valor pactado             | ARS 200,00 | ARS 100,00 | ARS 100,00 | 100 %             |
                | promedio                  | ARS 100,00 | ARS 100,00 | ARS 0,00   | 0 %               |

        @wip
        Scenario: 70.8-AP Comparar mis resultados cuando antes no había tenido trabajos
            Given que no tuve contrataciones, finalizaciones informadas ni cobros el 8 y 9 de septiembre de 2026
            And que el 10 y 11 de septiembre de 2026 me contrataron, informé la finalización y cobré un trabajo de ARS 100,01
            When comparo mis resultados del 10 y 11 de septiembre de 2026 con el período anterior
            Then la comparación corresponde al 8 y 9 de septiembre de 2026
            And veo una contratación, una finalización informada, un cobro completo y un cliente atendido más que antes
            And veo ARS 100,01 más de valor pactado que antes
            And no se calcula un cambio porcentual para ninguno de esos resultados porque antes eran cero
            And el promedio anterior y sus diferencias se muestran sin valor, en lugar de inventar un promedio

    Rule: Los pendientes muestran lo que debo atender ahora, aunque no haya tenido actividad en el período

        @wip
        Scenario: 70.9-AP Ver mis pendientes actuales aunque surgieron fuera del período consultado
            Given que tengo una solicitud pendiente de "beatriz@example.com" recibida antes del 20 de septiembre de 2026
            And que tengo otra solicitud pendiente de "diego@example.com" recibida el 23 de septiembre de 2026
            And que todavía figura por realizar un trabajo programado para el 15 de septiembre de 2026 y otro programado para el 1 de octubre, ambos acordados antes del 20 de septiembre
            And que informé la finalización de un trabajo antes del 20 de septiembre de 2026 que todavía espera el pago
            And que "pedro@example.com" tiene otra solicitud pendiente y otro trabajo que espera el pago
            When consulto mis resultados desde el 20 hasta antes del 22 de septiembre de 2026
            Then mis resultados de esos dos días son cero y el promedio no tiene valor
            And como no elegí otra forma de agruparlos, veo los dos días sin movimiento en mi evolución diaria
            And veo 2 solicitudes pendientes, 2 trabajos por realizar y 1 trabajo que espera el pago
            And no veo los pendientes de "pedro@example.com"

        @wip
        Scenario: 70.10-AP Ver resultados vacíos cuando todavía no tuve actividad
            Given que no tengo solicitudes ni trabajos
            When consulto mis resultados sin elegir un período
            Then veo mis resultados desde el 30 de agosto de 2026 a las 12:00 hasta el 29 de septiembre de 2026 a las 12:00
            And los resultados y pendientes son cero, mientras que el promedio no tiene valor
            And como no elegí otra forma de agruparlos, veo 31 días calendario sin movimiento, incluidos los días parciales del comienzo y del final
            And no veo una comparación con un período anterior

    Rule: Solo puedo consultar mis resultados con una identidad válida y un período admisible

        @wip
        Scenario Outline: 70.11-AP Rechazar períodos incompletos o fuera de los límites permitidos
            When intento consultar mis resultados <elección>
            Then se me informa que el período no es válido y no se muestran resultados

            Examples:
                | elección                                 |
                | sin indicar el comienzo                  |
                | sin indicar el final                     |
                | con el mismo comienzo y final            |
                | con un final anterior al comienzo        |
                | por más de 365 días                      |
                | con un final posterior al momento actual |

        @wip
        Scenario Outline: 70.12-AP Impedir la consulta sin una identidad de prestador válida
            Given que <identidad>
            When intento consultar mis resultados
            Then <resultado> y no se muestran resultados

            Examples:
                | identidad                                                   | resultado                                                       |
                | no inicié sesión                                            | se rechaza la consulta porque no inicié sesión                  |
                | mi sesión no es válida                                      | se rechaza la consulta porque mi sesión no es válida            |
                | estoy autenticado como consumidor "ana@example.com"         | se rechaza la consulta porque no soy prestador                  |
                | inicié sesión con una identidad que no figura en LoResuelvo | se rechaza la consulta porque no tengo una cuenta en LoResuelvo |
