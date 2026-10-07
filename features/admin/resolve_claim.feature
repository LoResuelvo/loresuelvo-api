Feature: Registrar un dictamen administrativo para un reclamo
    Como operador autorizado de soporte y mediación
    quiero registrar una resolución fundamentada
    para cerrar el expediente sin alterar la contratación ni ejecutar movimientos financieros

    Background:
        Given que existe el rubro "Plomería"
        And que existe el consumidor registrado "ana@example.com"
        And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que ambos participan en la solicitud de trabajo "S1"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: El dictamen finaliza el expediente según su tipo

        @wip
        Scenario Outline: 68.19-DC Registrar cada tipo de dictamen con su estado final
            Given que la fecha y hora actual del sistema es "2026-09-24T08:00:00-03:00"
            And que "ana@example.com" tiene el reclamo "C1" en estado "in_review"
            And que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When registro para "C1" el dictamen de tipo "<tipo>" con fundamentación "<fundamentación>", la clave nueva "550e8400-e29b-41d4-a716-446655440025" y la correlación "claim-resolution-1"
            Then el sistema responde con estado 200 y el reclamo queda en estado "<estado>"
            And la respuesta y el reclamo persistido conservan exactamente el tipo, la fundamentación y el instante "2026-09-25T12:00:00-03:00" registrado por el servidor
            And el expediente conserva su testimonio y referencias originales
            And queda registrada una sola actuación final
            And queda persistido un evento de ejecución exitoso del reclamo "C1" por "operador@example.com" en el instante "2026-09-25T12:00:00-03:00" y con la correlación "claim-resolution-1"
            And el evento sólo registra el cambio de estado de "in_review" a "<estado>", sin fundamentación ni compensación sugerida
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

            Examples:
                | tipo            | fundamentación                                  | estado    |
                | consumer_favor  | Se verificó el incumplimiento acordado         | resolved  |
                | provider_favor  | La evidencia confirma el cumplimiento ofrecido | resolved  |
                | agreement       | Ambas partes aceptaron el acuerdo registrado   | resolved  |
                | without_merit   | La evidencia no acredita mérito suficiente     | dismissed |

        @wip
        Scenario: 68.20-DC Registrar una compensación sugerida en ARS sin ejecutarla
            Given que la fecha y hora actual del sistema es "2026-09-21T10:55:00-03:00"
            And que existe la siguiente solicitud de trabajo:
                | solicitud | consumidor      | prestador        | creada                    | estado   | título          | descripción                |
                | S2        | ana@example.com | juan@example.com | 2026-09-20T09:00:00-03:00 | accepted | Reparar cañería | La canilla pierde agua.    |
            And que existe la siguiente propuesta de servicio:
                | propuesta | solicitud | creada                    | fecha programada           | duración | descripción                    | estado   | precio total | moneda | seña   | comisión total | comisión inicial | saldo servicio | saldo comisión |
                | P1        | S2        | 2026-09-21T09:00:00-03:00 | 2026-09-30T12:00:00-03:00 | 60       | Revisar y reparar la canilla. | accepted | 3333333      | ARS    | 777777 | 555555         | 123457           | 2555556        | 432098         |
            And que "P1" es la primera propuesta de la conversación de "S2"
            And que el intento de seña "I1" de "P1" tiene el estado "paid" y estos importes persistidos:
                | moneda | porción del prestador | comisión | total a pagar |
                | ARS    | 777777                | 123457   | 901234        |
            And que la fecha y hora actual del sistema es "2026-09-21T11:03:00-03:00"
            And que existe la siguiente orden de trabajo:
                | orden | propuesta | aceptada                  | estado           | finalización informada | saldo pagado |
                | O1    | P1        | 2026-09-21T11:03:00-03:00 | scheduled        |                        |              |
            And que la transacción externa "T1" de "I1" tiene el ID de Mercado Pago "mp-payment-9001", estado "approved", importe 901234 ARS y verificación "2026-09-21T11:03:00-03:00"
            And que "ana@example.com" tiene el reclamo "C1" en estado "in_review" sobre la primera propuesta "P1" de "S2"
            And que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When registro para "C1" un dictamen "agreement" con fundamentación "Las partes acordaron una compensación" y una compensación sugerida de 1500 centavos con la clave nueva "550e8400-e29b-41d4-a716-446655440026"
            Then el sistema responde con estado 200 y el expediente queda resuelto
            And el dictamen informa exactamente la compensación sugerida de 1500 centavos ARS
            And la compensación no se identifica como pagada ni ejecutada
            And la propuesta "P1" conserva sus términos en ARS: total de servicio 3333333, seña de servicio 777777, comisión total 555555 y comisión de seña 123457 centavos
            And la orden "O1" conserva el estado "scheduled" y no registra saldo pagado
            And el intento "I1" conserva el estado "paid" y la transacción "T1" conserva el estado "approved" y el importe 901234 centavos ARS
            And no se crea otro intento ni transacción de pago

    Rule: La fundamentación y la compensación deben ser válidas

        @wip
        Scenario Outline: 68.21-DC Rechazar una fundamentación vacía, excesiva o con texto inválido
            Given que "ana@example.com" tiene el reclamo "C1" en estado "in_review"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When intento dictaminar "C1" con <contenido> y una clave nueva
            Then el sistema responde con estado 400
            And el reclamo "C1" permanece en estado "in_review" sin dictamen ni nueva actuación

            Examples:
                | contenido                                          |
                | la fundamentación vacía                           |
                | una fundamentación de 5001 caracteres Unicode "ñ" |
                | sin el campo de fundamentación                     |

        @wip
        Scenario Outline: 68.22-DC Rechazar una compensación con importe ausente, no positivo, fraccionario o fuera de rango
            Given que "ana@example.com" tiene el reclamo "C1" en estado "in_review"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When intento registrar para "C1" una compensación sugerida <compensación> con una clave nueva
            Then el sistema responde con estado 400
            And el reclamo "C1" permanece en estado "in_review" sin dictamen ni nueva actuación

            Examples:
                | compensación                          |
                | de 0 centavos                         |
                | de -1 centavo                         |
                | de 1.5 centavos                       |
                | sin importe                           |
                | superior al máximo entero de 64 bits  |

        @wip
        Scenario Outline: 68.23-DC Rechazar una solicitud de dictamen con tipo o clave inválidos
            Given que "ana@example.com" tiene el reclamo "C1" en estado "in_review"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When intento registrar para "C1" un dictamen con <entrada>
            Then el sistema responde con estado 400
            And el reclamo "C1" permanece en estado "in_review" sin dictamen ni nueva actuación

            Examples:
                | entrada                                                                 |
                | el tipo no admitido "neutral", fundamentación válida y clave nueva     |
                | el tipo ausente, fundamentación válida y clave nueva                  |
                | tipo "consumer_favor", fundamentación válida y clave ausente         |
                | tipo "consumer_favor", fundamentación válida y clave "no-es-uuid"    |

    Rule: El dictamen sólo puede registrarse una vez y desde revisión

        @wip
        Scenario Outline: 68.24-DC Rechazar dictaminar un expediente fuera de revisión
            Given que "ana@example.com" tiene el reclamo "C1" en estado "<estado>"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When intento registrar un dictamen válido para "C1" con una clave nueva
            Then el sistema responde con estado 409
            And el reclamo "C1" conserva su estado y dictamen previos

            Examples:
                | estado    |
                | open      |
                | resolved  |
                | dismissed |

        @wip
        Scenario: 68.25-DC Recuperar el dictamen original al reintentar incluso después de finalizar
            Given que la fecha y hora actual del sistema es "2026-09-24T08:00:00-03:00"
            And que "ana@example.com" tiene el reclamo "C1" en estado "in_review" sobre la solicitud de trabajo "S1"
            And que "C1" conserva su referencia original a la solicitud de trabajo "S1"
            And que el administrador "operador@example.com" registró para "C1" el dictamen final "consumer_favor" con la clave "550e8400-e29b-41d4-a716-446655440032" en el instante "2026-09-24T10:00:00-03:00"
            And el reclamo "C1" conserva la fundamentación original y la fecha del dictamen
            And que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When repito el mismo dictamen para "C1" con la misma clave y contenido
            Then el sistema responde con estado 200 y el reclamo "C1" conserva el estado "resolved"
            And la respuesta recupera el dictamen, operador e instante originales
            And el reclamo conserva la referencia original a la solicitud de trabajo "S1"
            And no se duplican el dictamen, la actuación ni su auditoría

        @wip
        Scenario Outline: 68.26-DC Rechazar la reutilización de una clave para otra actuación o contenido
            Given que la fecha y hora actual del sistema es "2026-09-24T08:00:00-03:00"
            And que "ana@example.com" tiene el reclamo "C1" en estado "<estado_c1>" sobre "S1"
            And que "juan@example.com" tiene el reclamo "C2" en estado "<estado_c2>" sobre "S1"
            And que la fecha y hora actual del sistema es "2026-09-24T10:00:00-03:00"
            And que el administrador "operador@example.com" ya usó la clave "550e8400-e29b-41d4-a716-446655440033" para <uso_original>
            And que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_claims"
            When intento registrar para "C1" <uso_nuevo> con la misma clave
            Then el sistema responde con estado 409
            And no se registra un dictamen ni una actuación duplicada para "C1" o "C2"

            Examples:
                | uso_original                                         | estado_c1 | estado_c2 | uso_nuevo                                                  |
                | el dictamen "consumer_favor" del reclamo "C1"       | resolved  | in_review | el dictamen "provider_favor" con fundamentación distinta |
                | la revisión del reclamo "C1"                         | in_review | in_review | un dictamen "consumer_favor"                              |
                | el dictamen "consumer_favor" del reclamo "C2"       | in_review | resolved  | el dictamen "consumer_favor"                              |

    Rule: La escritura requiere el permiso administrativo específico

        @wip
        Scenario: 68.27-DC Rechazar el dictamen sin permiso de escritura de reclamos
            Given que "ana@example.com" tiene el reclamo "C1" en estado "in_review"
            And estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_claims"
            When intento registrar un dictamen válido para "C1" con una clave nueva
            Then el sistema responde con estado 403
            And el reclamo "C1" permanece en estado "in_review" sin dictamen ni nueva actuación
