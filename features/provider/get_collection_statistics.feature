Feature: Consultar mis cobros verificados y los saldos de mis trabajos
    Como prestador de LoResuelvo
    quiero conocer mis cobros verificados, los saldos pendientes y las operaciones que explican cada importe
    para entender qué dinero está registrado a mi favor sin confundirlo con las comisiones

    Background:
        Given que existen los consumidores "ana@example.com" y "carla@example.com"
        And que existen los prestadores "juan@example.com" y "pedro@example.com"
        And que hoy es 29 de septiembre de 2026 a las 12:00 en Buenos Aires
        And que estoy autenticado como prestador "juan@example.com"

    Rule: Solo cuentan mis cobros aprobados y verificados, por el importe contractual que me corresponde

        Scenario: 71.1-CP Separar la seña y el saldo de un trabajo sin sumar comisiones
            Given que acordé con "ana@example.com" un trabajo de ARS 1.000,00 con seña de ARS 200,00 y comisión total de ARS 50,00
            And que la seña se verificó el 2 de septiembre de 2026 por ARS 210,00, incluidos ARS 10,00 de comisión
            And que el saldo se verificó el 8 de septiembre de 2026 por ARS 840,00, incluidos ARS 40,00 de comisión
            When consulto mis cobros desde el 1 hasta antes del 10 de septiembre de 2026
            Then veo ARS 200,00 en señas, ARS 800,00 en saldos y ARS 1.000,00 en total, expresados en pesos argentinos y centavos enteros
            And el total coincide con la suma de señas y saldos
            And no veo datos de la cuenta de cobros ni de los medios de pago

        Scenario Outline: 71.2-CP Imputar cada cobro a la fecha en que se verificó
            Given que acordé con "ana@example.com" un trabajo de ARS 1.000,00 con seña de ARS 200,00 y comisión total de ARS 50,00
            And que inicié el pago de la seña el 1 de septiembre de 2026 a las 23:50 y se verificó el 2 de septiembre a las 00:10
            And que el saldo se verificó el 8 de septiembre de 2026
            When consulto mis cobros <período>
            Then veo <señas> en señas, <saldos> en saldos y <total> en total

            Examples:
                | período                                                     | señas      | saldos     | total      |
                | desde el 1 hasta antes del 2 de septiembre de 2026         | ARS 0,00   | ARS 0,00   | ARS 0,00   |
                | desde el 2 hasta antes del 5 de septiembre de 2026         | ARS 200,00 | ARS 0,00   | ARS 200,00 |
                | desde el 5 hasta antes del 10 de septiembre de 2026        | ARS 0,00   | ARS 800,00 | ARS 800,00 |

        Scenario: 71.3-CP Excluir pagos no aprobados sin perder cobros legítimos del mismo importe
            Given que tengo dos trabajos distintos con señas verificadas de ARS 100,00 cada una el 2 y el 3 de septiembre de 2026
            And que una de esas señas se aprobó en un nuevo intento después de uno rechazado el 1 de septiembre
            And que otra propuesta mía tiene un pago de seña iniciado el 2 de septiembre que sigue en procesamiento
            And que otra propuesta mía tiene un pago de seña iniciado el 2 de septiembre sin cobro aprobado
            When consulto mis cobros y su detalle desde el 1 hasta antes del 4 de septiembre de 2026
            Then veo exactamente ARS 200,00 en señas y ARS 0,00 en saldos
            And el detalle contiene solo dos cobros distintos de ARS 100,00

        Scenario: 71.4-CP No incluir cobros de otro prestador
            Given que "pedro@example.com" cobró una seña y un saldo entre el 1 y el 28 de septiembre de 2026
            And que yo cobré una seña de ARS 100,00 entre el 1 y el 28 de septiembre de 2026
            When consulto mis cobros y su detalle desde el 1 hasta antes del 29 de septiembre de 2026
            Then solo veo mis ARS 100,00 en el resumen y en el total del detalle
            And ninguna fila del detalle pertenece a "pedro@example.com"

    Rule: Puedo seguir los cobros en el tiempo y compararlos sin inventar actividad

        Scenario: 71.5-CP Ver días sin cobros y respetar el límite final del período
            Given que cobré una seña de ARS 100,00 el 1 de septiembre de 2026 a las 00:00
            And que cobré un saldo de ARS 300,00 el 2 de septiembre de 2026 a las 23:59:59, cuya seña cobré en agosto
            And que cobré otra seña el 4 de septiembre de 2026 a las 00:00
            When consulto mi evolución diaria desde el 1 hasta antes del 4 de septiembre de 2026
            Then veo exactamente estos días, en orden:
                | día             | señas      | saldos     | total      |
                | 1 de septiembre | ARS 100,00 | ARS 0,00   | ARS 100,00 |
                | 2 de septiembre | ARS 0,00   | ARS 300,00 | ARS 300,00 |
                | 3 de septiembre | ARS 0,00   | ARS 0,00   | ARS 0,00   |
            And la suma de esos días coincide con los importes del período

        Scenario: 71.6-CP Agrupar por semanas y recortar los extremos
            Given que cobré una seña de ARS 100,00 el domingo 6 de septiembre de 2026 a las 23:30
            And que cobré un saldo de ARS 300,00 el lunes 7 de septiembre de 2026 a las 00:30, cuya seña cobré en agosto
            When consulto mi evolución semanal desde el domingo 6 a las 12:00 hasta el martes 8 a las 12:00
            Then veo la seña en el tramo del domingo 6 a las 12:00 al lunes 7 a las 00:00
            And veo el saldo en el tramo del lunes 7 a las 00:00 al martes 8 a las 12:00
            And no veo otros tramos en el período elegido

        Scenario: 71.7-CP Agrupar por meses e incluir un tramo sin cobros
            Given que cobré una seña de ARS 100,00 el 6 de septiembre de 2026
            And que cobré un saldo de ARS 300,00 el 7 de septiembre de 2026, cuya seña cobré el 20 de agosto de 2026
            When consulto mi evolución mensual desde el 31 de agosto a las 12:00 hasta el 8 de septiembre a las 12:00
            Then veo agosto sin cobros y septiembre con ARS 100,00 en señas y ARS 300,00 en saldos

        Scenario: 71.8-CP Comparar los importes con un período anterior de igual duración
            Given que cobré ARS 100,00 en señas y ARS 300,00 en saldos durante el 8 y 9 de septiembre de 2026
            And que cobré ARS 200,00 en señas y ARS 600,00 en saldos durante el 10 y 11 de septiembre de 2026
            When comparo mis cobros del 10 y 11 de septiembre de 2026 con el período anterior
            Then la comparación corresponde al 8 y 9 de septiembre de 2026
            And veo estas diferencias entre ambos períodos:
                | concepto | actual     | anterior   | diferencia | cambio porcentual |
                | señas    | ARS 200,00 | ARS 100,00 | ARS 100,00 | 100 %             |
                | saldos   | ARS 600,00 | ARS 300,00 | ARS 300,00 | 100 %             |
                | total    | ARS 800,00 | ARS 400,00 | ARS 400,00 | 100 %             |

        Scenario: 71.9-CP No inventar un porcentaje cuando el período anterior no tuvo cobros
            Given que no tuve cobros el 8 y 9 de septiembre de 2026
            And que cobré una seña de ARS 100,01 durante el 10 y 11 de septiembre de 2026
            When comparo mis cobros del 10 y 11 de septiembre de 2026 con el período anterior
            Then veo ARS 100,01 más en señas y en total, y ARS 0,00 de diferencia en saldos
            And los cambios porcentuales de señas, saldos y total se muestran sin valor

    Rule: Los saldos pendientes describen mis trabajos actuales, no los cobros del período

        Scenario: 71.10-CP Ver los saldos de trabajos programados y finalizados aunque no haya un nuevo pago iniciado
            Given que tengo un trabajo programado de ARS 1.000,00 con seña de ARS 200,00 pagada antes del 20 de septiembre de 2026
            And que informé la finalización de otro trabajo de ARS 500,00 con seña de ARS 100,00 pagada antes del 20 de septiembre, cuyo saldo aún no se pagó
            And que las comisiones pactadas para esos trabajos suman ARS 50,00
            And que no se inició el pago de los saldos de esos trabajos
            And que tengo un tercer trabajo completamente pagado antes del 20 de septiembre de 2026
            And que "pedro@example.com" tiene un trabajo finalizado con pago pendiente
            When consulto mis cobros desde el 20 hasta antes del 22 de septiembre de 2026
            Then veo 1 trabajo programado con ARS 800,00 de saldo contractual pendiente
            And veo 1 trabajo finalizado con ARS 400,00 de saldo contractual pendiente
            And los cobros del período son cero, sin sumar comisiones ni pendientes de "pedro@example.com"

    Rule: El detalle explica los importes del conjunto filtrado, no solo los de una página

        Scenario: 71.11-CP Reconocer cada operación y conciliar el detalle completo con el resumen
            Given que tengo tres cobros verificados entre el 1 y el 28 de septiembre de 2026: dos señas de ARS 100,00 y un saldo de ARS 300,00
            When consulto mis cobros y su detalle desde el 1 hasta antes del 29 de septiembre de 2026
            Then el detalle informa 3 cobros y ARS 500,00 para el conjunto completo
            And cada cobro muestra su identificador local, fecha de verificación, concepto, importe, moneda y propuesta
            And cada cobro muestra también el trabajo al que corresponde
            And la suma de señas y saldos del resumen coincide con el total del detalle

        Scenario: 71.12-CP Filtrar y recorrer páginas sin cambiar el período ni los totales del filtro
            Given que tengo dos señas de ARS 100,00 verificadas el 3 y el 5 de septiembre de 2026, y un saldo de ARS 300,00 verificado el 6 de septiembre
            When recorro el detalle de señas desde el 1 hasta antes del 29 de septiembre de 2026, de a un cobro por página
            Then veo primero la seña más reciente y luego la anterior, sin repeticiones ni omisiones
            And cada página mantiene 2 cobros y ARS 200,00 para el conjunto filtrado
            And no aparece el saldo en ninguna de esas páginas

        Scenario: 71.13-CP Mantener fijo el período predeterminado al continuar una página
            Given que tengo dos cobros verificados dentro de los últimos 30 días, uno el 30 de agosto de 2026 a las 13:00 y otro el 28 de septiembre de 2026
            And que obtuve la primera página del detalle de a un cobro por página sin elegir período
            And que el reloj avanzó un día
            When continúo a la siguiente página del detalle
            Then la segunda página conserva el período de la primera y muestra el cobro restante

        Scenario: 71.14-CP Recibir una respuesta vacía aunque tenga saldos pendientes
            Given que no tuve cobros verificados en los últimos 30 días
            And que tengo un trabajo programado con ARS 800,00 de saldo contractual pendiente
            When consulto mis cobros y su detalle sin elegir un período
            Then veo mis cobros desde el 30 de agosto de 2026 a las 12:00 hasta el 29 de septiembre de 2026 a las 12:00
            And veo señas, saldos y total en cero, y 31 días calendario sin movimiento en la evolución diaria
            And veo el saldo pendiente del trabajo programado y ninguna comparación anterior
            And el detalle muestra cero cobros, importe total cero y ninguna fila

    Rule: Una identidad o consulta inválida no revela datos ni produce importes incompletos

        Scenario Outline: 71.15-CP Rechazar períodos y opciones no admitidos
            When intento consultar mis cobros <elección>
            Then se me informa que la consulta no es válida y no se muestran cobros

            Examples:
                | elección                                 |
                | sin indicar el comienzo                  |
                | sin indicar el final                     |
                | con el mismo comienzo y final            |
                | con un final anterior al comienzo        |
                | por más de 365 días                      |
                | con un final posterior al momento actual |
                | con una agrupación desconocida           |
                | con un concepto desconocido en el detalle |
                | con un tamaño de página mayor al permitido en el detalle |
                | con una continuación alterada en el detalle |
                | con una continuación de otro prestador en el detalle |
                | con una continuación combinada con opciones incompatibles en el detalle |

        Scenario Outline: 71.16-CP Impedir la consulta sin una identidad de prestador válida
            Given que <identidad>
            When intento consultar <vista>
            Then <resultado> y no se muestran cobros

            Examples:
                | identidad                                                   | vista       | resultado                                                       |
                | no inicié sesión                                            | el resumen  | se rechaza la consulta porque no inicié sesión                  |
                | mi sesión no es válida                                      | el detalle  | se rechaza la consulta porque mi sesión no es válida            |
                | estoy autenticado como consumidor "ana@example.com"         | el resumen  | se rechaza la consulta porque no soy prestador                  |
                | estoy autenticado como consumidor "ana@example.com"         | el detalle  | se rechaza la consulta porque no soy prestador                  |
                | inicié sesión con una identidad que no figura en LoResuelvo | el resumen  | se rechaza la consulta porque no tengo una cuenta en LoResuelvo |
                | inicié sesión con una identidad que no figura en LoResuelvo | el detalle  | se rechaza la consulta porque no tengo una cuenta en LoResuelvo |
