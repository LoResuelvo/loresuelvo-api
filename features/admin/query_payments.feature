Feature: Consultar pagos administrativos y su desglose económico
    Como operador autorizado de LoResuelvo
    quiero localizar los intentos y transacciones de pago con su desglose económico
    para investigar reservas y saldos pendientes sin confundir un checkout iniciado con un cobro acreditado

    Background:
        Given que existe el rubro "Plomería"
        And que existen los siguientes consumidores registrados:
            | correo              | nombre  | apellido |
            | ana@example.com     | Ana     | Pérez    |
            | beatriz@example.com | Beatriz | Suárez   |
            | carla@example.com   | Carla   | Gómez    |
        And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "López" y rubro "Plomería"
        And existe un prestador registrado con correo "luis@example.com", nombre "Luis", apellido "Ruiz" y rubro "Plomería"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "Díaz"

    Rule: Cada resultado representa un intento y conserva la evidencia financiera disponible
        Background:
            Given que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_payments"

        @wip
        Scenario: 65.1-PA Desglosar la seña con los importes contractuales persistidos
            Given que la propuesta "P1" de "ana@example.com" con "juan@example.com" conserva estos términos en centavos:
                | moneda | total del servicio | seña del servicio | comisión total | comisión de la seña |
                | ARS    | 3333333            | 777777            | 555555         | 123457              |
            And que la orden "O1" corresponde a la propuesta "P1"
            And que el intento de seña "I1" de "P1" tiene el estado "paid" y estos importes persistidos:
                | moneda | porción del prestador | comisión | total a pagar |
                | ARS    | 777777                | 123457   | 901234        |
            And que la transacción externa "T1" de "I1" tiene el ID de Mercado Pago "mp-payment-9001", estado "approved", importe 901234 ARS y verificación "2026-09-20T12:00:00Z"
            And que al iniciar el checkout se envió el UUID interno de "I1" como valor de "external_reference" a Mercado Pago, distinto del ID externo "mp-payment-9001"; el valor devuelto por Mercado Pago no se afirma como persistido
            When consulto los pagos administrativos con correlación "admin-payments-deposit-1"
            Then el sistema responde con estado 200
            And la colección contiene el intento "I1" una sola vez, vinculado a "P1", a la orden "O1", al consumidor "ana@example.com" y al prestador "juan@example.com" mediante sus IDs internos
            And el intento informa el propósito "booking_deposit", su estado "paid", moneda "ARS" y creación persistida
            And el importe aprobado bruto es exactamente 901234 centavos ARS, no representa liquidación ni ingreso neto y está respaldado por la transacción aprobada coincidente de "I1"
            And el desglose en centavos informa 3333333 de servicio, 777777 de seña del servicio, 123457 de comisión de la seña, 901234 a pagar en la reserva, 2555556 de saldo restante pactado del servicio, 432098 de comisión restante pactada y 2987654 de total restante pactado; estos saldos son contractuales y no afirman cobros pendientes
            And los importes del intento separan la porción contractual del prestador y la comisión de plataforma, y su suma coincide con el total de 901234
            And no se publican porcentajes recomputados si no forman parte de los términos persistidos
            And la transacción conserva el ID externo, el estado, la moneda, el importe y el instante de verificación persistidos
            And como las transacciones persistidas no contienen comisión del procesador ni neto liquidado al prestador, esos campos se informan como no disponibles, no como cero, y no se afirma que los fondos se hayan liquidado
            And la respuesta no expone URLs de checkout, preferencias externas, credenciales OAuth, tokens, payloads crudos ni datos completos de instrumentos de pago
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        @wip
        Scenario: 65.2-PA Desglosar un intento de saldo sin inventar evidencia de cobro ni liquidación
            Given que la propuesta "P1" de "ana@example.com" con "juan@example.com" conserva estos términos en centavos:
                | moneda | total del servicio | seña del servicio | comisión total | comisión de la seña |
                | ARS    | 3333333            | 777777            | 555555         | 123457              |
            And que el intento de seña "I0" de "P1" fue rechazado antes de iniciar nuevos intentos y no tiene transacción externa
            And que la orden existente "O1" corresponde a "P1", está en estado "awaiting_payment" y tiene la seña pagada "I1" por 901234 centavos ARS con exactamente una transacción externa aprobada y coincidente
            And que el intento de saldo "I2" de "O1" tiene el estado "checkout_ready", su sesión de checkout está vigente y estos importes persistidos:
                | moneda | porción del prestador | comisión | total a pagar |
                | ARS    | 2555556               | 432098   | 2987654       |
            And que "I2" no tiene transacciones externas persistidas
            When consulto los pagos administrativos con correlación "admin-payments-balance-1"
            Then el sistema responde con estado 200
            And la colección contiene el intento "I2" una sola vez, vinculado a "P1", a la orden "O1", al consumidor "ana@example.com" y al prestador "juan@example.com"
            And el desglose informa el total de servicio, la seña, la comisión de la seña y el saldo pactado en los términos persistidos de "P1", sin aplicar porcentajes actuales
            And el intento de saldo separa 2555556 centavos destinados contractualmente al prestador y 432098 de comisión, cuya suma es el total de 2987654 centavos ARS
            And la colección de transacciones externas de "I2" está vacía y no se presenta la sesión de checkout como evidencia de cobro
            And el neto liquidado y las comisiones del procesador se informan como no disponibles, no como cero
            And el balance pendiente deriva de los términos válidos de "P1", la seña pagada y el estado contractual vigente de "O1", no de porcentajes actuales ni de la sesión de checkout
            And el resumen a nivel de propuesta "P1" y orden "O1" informa 901234 centavos ARS aprobados brutos por la seña aprobada, sin afirmar liquidación ni ingreso neto, y 2987654 centavos ARS de importe pendiente de cobro por el saldo contractual vigente
            And esos importes corresponden al resumen de "P1"/"O1", no a un total global de la colección ni a la suma de importes de la fila del intento "I2"
            And ni la seña rechazada "I0" ni el intento de saldo "I2" sin transacción aprobada se suman al importe acreditado

        @wip
        Scenario: 65.3-PA Separar el estado del intento de la evidencia de cobro
            Given que la propuesta "P1" de "ana@example.com" con "juan@example.com" conserva términos de contratación válidos
            And que en secuencia "I1" de seña expiró sin transacción externa, "I2" de seña fue rechazado y "I3" de seña está en "processing"; cada intento anterior quedó terminal antes de crear el siguiente
            And que "I2" tiene una transacción externa persistida con el ID de Mercado Pago "9020", estado "rejected", importe 901234 ARS
            And que "I3" tiene una transacción externa persistida con el ID de Mercado Pago "9021", estado "processing", importe 901234 ARS, y conserva una URL de checkout y una preferencia externa
            When consulto los pagos administrativos
            Then el sistema responde con estado 200
            And aparecen los tres intentos por separado con sus estados persistidos y sin una transacción aprobada
            And el importe aprobado bruto es exactamente 0 centavos ARS porque no hay transacciones aprobadas persistidas
            And las transacciones persistidas "9020" y "9021" se muestran con sus estados "rejected" y "processing" y ninguna se informa como importe acreditado
            And la URL y la preferencia no se presentan como acreditación ni como importe liquidado
            And cada intento conserva el vínculo con "P1" y el consumidor y prestador correctos, e informa la orden como nula si todavía no existe

        @wip
        Scenario: 65.4-PA Conservar cada intento de seña creado tras un rechazo
            Given que la propuesta "P1" de "ana@example.com" con "juan@example.com" conserva términos de contratación válidos
            And que "I1" fue creado el "2026-09-20T12:00:00Z" y quedó rechazado antes de crear el siguiente intento
            And que "I2" fue creado el "2026-09-20T12:06:00Z" y quedó pagado el "2026-09-20T12:10:00Z"
            And que "I2" tiene la transacción aprobada "mp-payment-9002" por el importe total de su intento
            When consulto los pagos administrativos por la propuesta "P1"
            Then el sistema responde con estado 200
            And aparecen exactamente los intentos "I2" y "I1" en ese orden, cada uno una sola vez y con su propio estado, importe y fecha
            And sólo "I2" contiene la evidencia aprobada "mp-payment-9002" y no se atribuye esa transacción a "I1"

        @wip
        Scenario: 65.5-PA Conservar todas las transacciones externas asociadas a un mismo intento
            Given que la propuesta "P1" de "ana@example.com" con "juan@example.com" conserva términos de contratación válidos
            And que el intento de seña "I1" de "P1" está pagado
            And que las dos transacciones aprobadas siguientes son una anomalía histórica preexistente persistida directamente, no el resultado del flujo normal de notificaciones
            And que "I1" tiene las siguientes transacciones externas persistidas:
                | ID de Mercado Pago | estado   | moneda | importe | verificada            |
                | 9003              | approved | ARS    | 901234  | 2026-09-20T12:00:00Z |
                | 9004              | approved | ARS    | 901234  | 2026-09-20T12:01:00Z |
            When consulto los pagos administrativos por el ID exacto de "I1"
            Then el sistema responde con estado 200
            And la respuesta contiene una sola fila para el intento "I1" y conserva ambas transacciones externas, sus IDs y sus instantes
            And cada transacción queda vinculada a "I1" y a la misma propuesta, sin multiplicar la fila del intento
            And la existencia de más de una transacción aprobada se identifica como una inconsistencia para investigar y no elimina ni reemplaza ninguna evidencia
            And el importe aprobado bruto es 1802468 centavos ARS, sumando una vez cada ID externo distinto del procesador, sin afirmar liquidación ni ingreso neto
            And el exceso sobre el importe contractual se identifica para investigación, sin restarlo del saldo contractual pendiente
            And el intento, sus términos, sus transacciones y la orden relacionada permanecen sin modificaciones

        @wip
        Scenario: 65.6-PA Identificar importes discordantes sin corregirlos durante la consulta
            Given que la propuesta "P1" de "ana@example.com" con "juan@example.com" conserva términos contractuales en ARS
            And que existe una inconsistencia histórica persistida directamente: los términos de "P1" pactan una seña de 777777 centavos ARS y una comisión de seña de 123457 centavos ARS, el intento "I1" de "P1" figura pagado por 901234 centavos ARS y su transacción figura aprobada por 901235 centavos USD
            When consulto los pagos administrativos por el ID exacto de "I1"
            Then el sistema responde con estado 200
            And la respuesta identifica la discrepancia preexistente de moneda e importe entre los términos, el intento y la transacción para investigación, sin corregirla ni conciliarla
            And los importes y monedas se muestran como persistidos, en centavos enteros, sin conversión ni recomposición con coma flotante
            And el importe aprobado bruto es 901235 centavos USD, separado por moneda, sin esconder la discrepancia ni convertirlo o restarlo del saldo contractual en ARS
            And la consulta no modifica ni intenta conciliar el intento, la transacción, la propuesta o la orden

        @wip
        Scenario: 65.7-PA No duplicar una transacción cuando se recibe más de una vez la misma notificación
            Given que la propuesta "P1" de "ana@example.com" con "juan@example.com" tiene el intento pagado "I1"
            And que durante la preparación del fixture se envía dos veces la misma notificación aprobada de Mercado Pago con ID "mp-payment-9005"
            And que antes de la consulta el repositorio conserva una sola transacción con el ID externo "mp-payment-9005" asociada a "I1"
            When consulto los pagos administrativos por el ID exacto de "I1"
            Then el sistema responde con estado 200
            And "I1" aparece una sola vez y su evidencia contiene una sola transacción con el ID "mp-payment-9005"
            And la consulta no crea ni modifica intentos, transacciones, propuestas ni órdenes

    Rule: Los filtros identifican sin ambigüedad cada intento y combinan criterios
        Los estados publicados y filtrables son requires_checkout, checkout_ready, processing, paid, rejected, expired, cancelled, refunded, disputed y payment_mismatch.
        La consulta histórica no agrega transiciones al flujo actual de pagos.
        Background:
            Given que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_payments"

        @wip
        Scenario Outline: 65.8-PA Buscar IDs exactos sin confundir el intento, el pago externo y la propuesta
            Given que la propuesta "P1" tiene la seña rechazada "I1" y la propuesta "P2" tiene la orden existente "O2" en estado "awaiting_payment", con la seña pagada "I2" y el intento de saldo "I3"
            And "I2" es un intento "booking_deposit" en estado "paid" con una transacción persistida aprobada, ID de Mercado Pago "9010"; "I3" es "service_balance", está en "checkout_ready" y no tiene transacción externa
            And al iniciar el checkout de "I2" se envió su UUID interno como "external_reference"; el valor devuelto por Mercado Pago no se persiste en la transacción
            When consulto los pagos administrativos con el parámetro "<parámetro>" igual a "<valor>"
            Then el sistema responde con estado 200
            And la página contiene exactamente el conjunto de intentos "<intentos>", sin asumir un orden no definido para esta consulta
            And el resultado específico de esta consulta es "<detalle>"

            Examples:
                | parámetro           | valor             | intentos | detalle                                                                                           |
                | payment_intent_id   | I1                | I1       | coincide exactamente con el ID interno del intento I1                                           |
                | external_reference  | I2                | I2       | busca el UUID interno canónico de I2, que fue el valor enviado a Mercado Pago como external_reference |
                | external_payment_id | 9010              | I2       | incluye el intento I2 completo y su transacción 9010                                  |
                | service_proposal_id | P2                | I2, I3   | incluye el conjunto de intentos I2 e I3 de P2, cada uno con su propia evidencia, sin asumir orden                                  |
                | external_payment_id | 901               | ninguno  | no encuentra coincidencias con un prefijo del ID externo 9010                         |

        @wip
        Scenario: 65.9-PA Buscar por correo con texto parcial sin distinguir mayúsculas
            Given que existen intentos de pago para "ana@example.com" con "juan@example.com", para "beatriz@example.com" con "juan@example.com" y para "ana@example.com" con "luis@example.com"
            When consulto los pagos administrativos con los filtros de correo "ANA@EXAMPLE" para consumidor y "JUAN@EXAMPLE" para prestador
            Then el sistema responde con estado 200
            And la página contiene únicamente el intento asociado a "ana@example.com" y "juan@example.com"
            And la búsqueda por correo admite coincidencia textual parcial sin distinguir mayúsculas, mientras los identificadores se buscan de manera exacta

        @wip
        Scenario: 65.10-PA Combinar propósito, estado y rango temporal del intento
            Given que hay intentos de seña pagados creados en "2026-09-20T10:00:00Z" y "2026-09-21T00:00:00Z", un intento de seña rechazado creado en "2026-09-20T12:00:00Z" y un intento de saldo pagado creado en "2026-09-20T13:00:00Z"
            And la transacción aprobada del intento creado exactamente al inicio del rango se verificó el "2026-09-22T00:00:00Z", fuera del rango de creación consultado
            When consulto los pagos administrativos con propósito "booking_deposit", estado del intento "paid", desde "2026-09-20T10:00:00Z" hasta "2026-09-21T00:00:00Z"
            Then el sistema responde con estado 200
            And la página contiene únicamente el intento pagado creado exactamente al inicio del rango
            And todos los filtros se combinan mediante AND y el fin del rango es exclusivo
            And el rango temporal se aplica a la fecha de creación del intento, no a la verificación de sus transacciones

        @wip
        Scenario Outline: 65.11-PA Rechazar filtros y límites que no pertenecen al contrato
            When consulto los pagos administrativos con la consulta "<consulta>"
            Then el sistema responde con estado 400
            And la respuesta no contiene intentos ni desglose económico
            And no se registra un evento de acceso a los pagos para la consulta inválida

            Examples:
                | consulta                                      |
                | payment_intent_id=no-es-uuid                 |
                | service_proposal_id=0                        |
                | purpose=refund                               |
                | intent_status=approved                       |
                | created_from=no-es-fecha                     |
                | created_to=no-es-fecha                       |
                | created_from=2026-09-21T00:00:00Z&created_to=2026-09-20T00:00:00Z |
                | limit=0                                      |
                | limit=101                                    |
                | cursor=alterado                              |

    Rule: La paginación devuelve páginas completas, ordenadas y estables
        Background:
            Given que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_payments"

        @wip
        Scenario: 65.12-PA Entregar una colección vacía y documentar el límite predeterminado
            Given que no hay intentos de pago que coincidan con la consulta
            When consulto los pagos administrativos sin indicar un límite y con correlación "admin-payments-empty-1"
            Then el sistema responde con estado 200
            And la página contiene la colección "payments" vacía y no nula, el límite predeterminado 20 y el cursor siguiente nulo
            And queda persistido exactamente un evento de acceso preparado al recurso "payment" por el operador "operador@example.com" con la correlación "admin-payments-empty-1"
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        @wip
        Scenario: 65.13-PA Recorrer páginas sin repetir intentos cuando coinciden sus fechas de creación
            Given que existen tres intentos con la misma fecha de creación y los IDs persistidos ordenados "I3", "I2" e "I1" de mayor a menor
            And que obtuve la primera página de pagos con límite 2, correlación "admin-payments-page-1" y guardé su cursor
            When continúo los pagos administrativos con el cursor guardado y correlación "admin-payments-page-2"
            Then el sistema responde con estado 200
            And la primera página contiene "I3" e "I2" y la siguiente contiene únicamente "I1"
            And las páginas respetan el orden estable por fecha de creación descendente e ID de intento descendente, sin omisiones ni repeticiones
            And la continuación conserva el límite 2 y los filtros de la primera página
            And queda persistido exactamente un evento de acceso preparado al recurso "payment" para cada página exitosa, uno con cada correlación

        @wip
        Scenario: 65.14-PA Aceptar el máximo de cien resultados por página
            Given que existen 101 intentos de pago que coinciden con la consulta
            When consulto los pagos administrativos con límite 100
            Then el sistema responde con estado 200
            And la página contiene exactamente cien intentos y el límite efectivo es 100
            And el cursor siguiente no es nulo porque queda un intento coincidente fuera de esta página

        @wip
        Scenario: 65.15-PA Rechazar un cursor manipulado o usado con otros filtros
            Given que hay varios intentos para las propuestas "P1" y "P2"
            And que obtuve una página y un cursor válido al filtrar por la propuesta "P1"
            When continúo la consulta con ese cursor y el filtro de propuesta "P2"
            Then el sistema responde con estado 400
            And la respuesta no contiene intentos ni desglose económico

    Rule: La consulta se autoriza y registra antes de entregar datos sensibles
        Background:
            Given que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_payments"

        @wip
        Scenario: 65.16-PA Auditar una sola vez la consulta de la colección antes de entregar sus filas
            Given que existen dos intentos de pago administrativos y no hay evento con la correlación "admin-payments-audit-1"
            When consulto los pagos administrativos con correlación "admin-payments-audit-1"
            Then el sistema responde con estado 200
            And antes de entregar la página queda persistido exactamente un evento de acceso preparado al recurso "payment" sin ID de fila, con el operador "operador@example.com" y la correlación "admin-payments-audit-1"
            And el evento de auditoría no requiere motivo manual ni incluye filtros, correos, montos o contenido de la respuesta
            And no se registra un evento separado por cada intento o transacción de la página
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        @wip
        Scenario: 65.17-PA No entregar resultados si falla la persistencia de auditoría
            Given que existe al menos un intento de pago que coincide con la consulta
            And que falla el almacenamiento del evento de auditoría de esta consulta
            When consulto los pagos administrativos con correlación "admin-payments-audit-failure-1"
            Then el sistema responde con estado 500
            And la respuesta no contiene intentos, transacciones ni desglose económico parcial
            And no queda persistido un evento de acceso para la consulta que falló al auditar
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        @wip
        Scenario: 65.18-PA No convertir un fallo de lectura en una página vacía ni en saldo cero
            Given que falla la lectura persistida de los pagos que coinciden con la consulta
            When consulto los pagos administrativos
            Then el sistema responde con estado 500
            And la respuesta no contiene intentos, transacciones ni importes presentados como cero
            And no se registra un evento de acceso preparado para una página que no pudo leerse
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

    Rule: Las rutas de pagos mantienen su autorización específica
        @wip
        Scenario Outline: 65.19-PA Rechazar el acceso administrativo sin autenticación o permiso
            Given que <autenticación>
            When intento consultar los pagos administrativos
            Then el sistema responde con estado <estado>
            And la respuesta no contiene intentos ni desglose económico
            And no se registra ningún evento de acceso a pagos

            Examples:
                | autenticación                                                                  | estado |
                | no envío un token Bearer                                                       | 401    |
                | envío un token Bearer inválido                                                  | 401    |
                | estoy autenticado como administrador "operador@example.com" sólo con "read:admin_operations" | 403 |
