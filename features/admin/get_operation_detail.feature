Feature: Consultar el detalle administrativo de una operación
    Como operador de LoResuelvo
    quiero consultar la evidencia persistida de una contratación
    para revisar su contexto, evolución y referencias sin intervenir en ella

    Background:
        Given que existen los rubros "Plomería" y "Electricidad"
        And que existen los siguientes consumidores registrados:
            | correo          | nombre | apellido |
            | ana@example.com | Ana    | Pérez    |
        And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"
        And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"

    Rule: El detalle corresponde a una identidad estable y sólo agrega evidencia de la operación consultada

        @wip
        Scenario: 64.1.1-OD Mostrar solicitud, evaluación persistida, procedencia de contacto y sus imágenes
            Given que el consumidor "ana@example.com" tiene la dirección actual "Av. Rivadavia", número "1420", piso "3", unidad "B"
            And que existe la conversación "C1" del consumidor "ana@example.com" con los siguientes mensajes persistidos:
                | mensaje | enviada              | contenido                                  |
                | M2      | 2026-09-20T12:13:00Z | Hay una pérdida debajo de la pileta.      |
                | M3      | 2026-09-20T12:15:30Z | ¿Puedo limpiar también el filtro?         |
            And que existen las siguientes evaluaciones persistidas de conversación:
                | evaluación | conversación | consumidor      | creada               | versión | resultado             | rubro    | título                 | descripción                                      | mensaje base |
                | A1         | C1           | ana@example.com | 2026-09-20T12:14:00Z | 2       | professional_required | Plomería | Pérdida bajo la pileta | Hay una pérdida debajo de la pileta de la cocina. | M2           |
                | A2         | C1           | ana@example.com | 2026-09-20T12:16:00Z | 3       | self_service          |          | Limpiar filtro         | El filtro puede limpiarse sin asistencia.          | M3           |
            And que existe la siguiente solicitud de trabajo:
                | solicitud | conversación | evaluación origen | consumidor      | prestador        | creada               | estado  | título                 | descripción                                      |
                | S1        | C1           | A1                | ana@example.com | juan@example.com | 2026-09-20T12:15:00Z | pending | Pérdida bajo la pileta | Hay una pérdida debajo de la pileta de la cocina. |
            And que "S1" tiene las siguientes imágenes persistidas:
                | archivo   | nombre original | mime_type  | visibilidad | propósito          | creado               |
                | request-1 | canilla.jpg     | image/jpeg | private      | job_request_image | 2026-09-20T12:14:00Z |
            When consulto el detalle administrativo de la operación de la solicitud "S1"
            Then el sistema responde con estado 200
            And el detalle informa el identificador "jr-" seguido del ID persistido de "S1"
            And la sección de solicitud informa el ID de "S1", estado "pending", título, descripción y el instante de creación "2026-09-20T12:15:00Z"
            And el detalle informa la evaluación persistida "A1" versión 2, basada en el mensaje "M2", con su resultado, rubro, título, descripción y el instante "2026-09-20T12:14:00Z"
            And la evaluación expuesta coincide con el origen guardado en "S1" aunque exista una evaluación más reciente en la conversación
            And el contacto informa al consumidor "Ana Pérez" y al prestador "Juan Gómez" con sus IDs internos
            And la dirección informa los campos guardados "Av. Rivadavia", "1420", "3" y "B" con procedencia de la dirección actual del consumidor
            And la imagen de la solicitud informa el archivo "request-1", nombre "canilla.jpg" y propósito "job_request_image"
            And el detalle no expone mensajes ni contenido de la conversación
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        Scenario: 64.1.2-OD Separar propuestas relacionadas y limitar cronología y pagos a la operación elegida
            Given que existe la siguiente solicitud de trabajo:
                | solicitud | consumidor      | prestador        | creada               | estado   | título            | descripción              |
                | S1        | ana@example.com | juan@example.com | 2026-09-10T13:00:00Z | accepted | Reparar calefón | El calefón no enciende. |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada               | fecha programada     | duración | descripción                         | estado   | precio total | moneda | seña | comisión total | comisión inicial | saldo servicio | saldo comisión |
                | P1        | S1        | 2026-09-11T13:00:00Z | 2026-09-20T13:00:00Z | 90       | Revisar calefón y cambiar piloto. | accepted | 8500000      | ARS    | 1700000 | 500000         | 100000           | 6800000        | 400000         |
                | P2        | S1        | 2026-09-19T13:00:00Z | 2026-09-29T13:00:00Z | 120      | Diagnosticar y reparar calefón.   | pending  | 12000000     | ARS    | 2400000 | 500000         | 100000           | 9600000        | 400000         |
            And que existen las siguientes órdenes de trabajo:
                | orden | propuesta | aceptada             | estado           | finalización informada | saldo pagado |
                | O1    | P1        | 2026-09-12T13:00:00Z | awaiting_payment | 2026-09-20T15:00:00Z   |              |
            And que "P1" tiene los siguientes intentos de pago de seña:
                | referencia interna | propósito      | estado   | creada               |
                | pi-deposit-1       | booking_deposit | rejected | 2026-09-12T13:01:00Z |
                | pi-deposit-2       | booking_deposit | paid     | 2026-09-12T13:03:00Z |
            And que "O1" tiene el siguiente intento de pago de saldo:
                | referencia interna | propósito       | estado         | creada               |
                | pi-balance-1       | service_balance | checkout_ready | 2026-09-20T15:01:00Z |
            When consulto el detalle administrativo de la operación "jr-" seguido del ID persistido de "S1"
            Then el sistema responde con estado 200
            And el detalle primario informa la identidad "jr-" seguida del ID persistido de "S1" y la solicitud "S1"
            And las propuestas relacionadas aparecen separadas con identidades "jr-" seguido del ID persistido de "S1" para "P1" y "sp-" seguido del ID persistido de "P2"
            And la propuesta "P1" informa estado "accepted", descripción, precio total 8500000 y moneda "ARS", creación "2026-09-11T13:00:00Z", fecha programada "2026-09-20T13:00:00Z" y duración 90 minutos
            And la propuesta "P2" informa estado "pending", descripción, precio total 12000000 y moneda "ARS", creación "2026-09-19T13:00:00Z", fecha programada "2026-09-29T13:00:00Z" y duración 120 minutos
            And cada propuesta informa sus términos de contratación persistidos: seña, comisión total e inicial, saldo de servicio y saldo de comisión, sin recalcularlos desde la propuesta hermana
            And la cronología y los hitos de pago principales contienen solamente los de "P1" y su orden "O1"
            And los hitos incluyen las referencias internas de ambos intentos de seña y del intento de saldo con propósito, estado e instante, sin confundir sus estados con los de transacciones ni exponer montos, URLs de checkout, tokens o payloads del proveedor
            And la orden "O1" informa estado "awaiting_payment", instante de aceptación "2026-09-12T13:00:00Z", finalización informada "2026-09-20T15:00:00Z" e instante de saldo pagado nulo
            And la respuesta no contiene mensajes ni extractos de chat

        Scenario: 64.1.3-OD Consultar una propuesta posterior por su identidad y limitar sus hitos
            Given que existe la siguiente solicitud de trabajo:
                | solicitud | consumidor      | prestador        | creada               | estado   | título            | descripción             |
                | S1        | ana@example.com | juan@example.com | 2026-09-10T13:00:00Z | accepted | Reparar calefón | El calefón no enciende. |
            And que existen las siguientes propuestas de servicio:
                | propuesta | solicitud | creada               | fecha programada     | duración | descripción                    | estado   | precio total | moneda | seña | comisión total | comisión inicial | saldo servicio | saldo comisión |
                | P1        | S1        | 2026-09-11T13:00:00Z | 2026-09-20T13:00:00Z | 90       | Revisar calefón.              | pending  | 8500000      | ARS    | 1700000 | 500000         | 100000           | 6800000        | 400000         |
                | P2        | S1        | 2026-09-19T13:00:00Z | 2026-09-29T13:00:00Z | 120      | Reemplazar válvula y probar.  | accepted | 12000000     | ARS    | 2400000 | 500000         | 100000           | 9600000        | 400000         |
            And que "P1" tiene el siguiente intento de pago de seña:
                | referencia interna | propósito      | estado   | creada               |
                | pi-deposit-1       | booking_deposit | rejected | 2026-09-12T13:03:00Z |
            And que existe la siguiente orden de trabajo:
                | orden | propuesta | aceptada             | estado    | finalización informada | saldo pagado |
                | O2    | P2        | 2026-09-20T13:00:00Z | scheduled |                        |              |
            And que "P2" tiene el siguiente intento de pago de seña:
                | referencia interna | propósito      | estado   | creada               |
                | pi-deposit-2       | booking_deposit | paid     | 2026-09-20T13:01:00Z |
            When consulto el detalle administrativo de la operación "sp-" seguido del ID persistido de "P2"
            Then el sistema responde con estado 200
            And el detalle primario informa la propuesta "P2" con identidad "sp-" seguida del ID persistido de "P2"
            And la propuesta hermana "P1" se informa por separado con identidad "jr-" seguida del ID persistido de "S1"
            And la cronología principal incluye únicamente los eventos de "P2" y su orden "O2", no los eventos de "P1"
            And los hitos de pago principales incluyen "pi-deposit-2" y no incluyen la referencia hermana "pi-deposit-1"
            And la propuesta primaria conserva sus términos persistidos de seña 2400000, comisión total 500000, comisión inicial 100000, saldo de servicio 9600000 y saldo de comisión 400000

    Rule: La orden expone evidencia persistida de finalización y review sin revelar los medios privados

        @wip
        Scenario: 64.1.4-OD Mostrar reporte y review persistidos de la orden
            Given que existe la siguiente solicitud de trabajo:
                | solicitud | consumidor      | prestador        | creada               | estado   | título            | descripción             |
                | S1        | ana@example.com | juan@example.com | 2026-09-01T13:00:00Z | accepted | Pintar dormitorio | Pintura descascarada. |
            And que existe la siguiente propuesta de servicio:
                | propuesta | solicitud | creada               | fecha programada     | duración | descripción       | estado   | precio total | moneda |
                | P1        | S1        | 2026-09-02T13:00:00Z | 2026-09-10T13:00:00Z | 180      | Preparar y pintar. | accepted | 25000000     | ARS    |
            And que existe la siguiente orden de trabajo:
                | orden | propuesta | aceptada             | estado | finalización informada | saldo pagado          |
                | O1    | P1        | 2026-09-03T13:00:00Z | paid   | 2026-09-10T17:00:00Z   | 2026-09-11T13:00:00Z |
            And que el reporte persistido de "O1" tiene descripción "Se lijaron y pintaron dos manos." e imágenes privadas "completion-1" y "completion-2" creadas el "2026-09-10T17:00:00Z"
            And que la review persistida de "O1" tiene calificación 5 y descripción "Quedó impecable y limpio."
            When consulto el detalle administrativo de la operación de la orden "O1"
            Then el sistema responde con estado 200
            And la orden "O1" informa el estado "paid", instante de finalización informada "2026-09-10T17:00:00Z" e instante de saldo pagado "2026-09-11T13:00:00Z"
            And el reporte informa la descripción persistida, el instante "2026-09-10T17:00:00Z" y exactamente las imágenes "completion-1" y "completion-2"
            And la review informa calificación 5 y la descripción persistida, sin atribuirle un timestamp no persistido
            And las imágenes se entregan como medios privados accesibles mediante autorización administrativa, incluso si el mecanismo usa URLs firmadas de corta duración
            And la respuesta no expone mensajes ni contenido de chat

    Rule: Los datos ausentes se distinguen de los fallos y la consulta conserva una cronología veraz
        Background:
            Given que existe la siguiente solicitud de trabajo sin referencia persistida a una evaluación de origen:
                | solicitud | consumidor      | prestador        | creada               | estado  | título               | descripción             |
                | S1        | ana@example.com | juan@example.com | 2026-09-20T12:00:00Z | pending | Revisar instalación | Hay una falla eléctrica. |

        @wip
        Scenario: 64.1.5-OD Informar datos ausentes cuando no hay relaciones persistidas
            Given que no existen dirección, propuestas, órdenes, reportes, reviews ni referencias de pago persistidas para "S1"
            When consulto el detalle administrativo de la operación de la solicitud "S1"
            Then el sistema responde con estado 200
            And el detalle informa el ID estable "jr-" seguido del ID persistido de "S1" y conserva los datos persistidos de solicitud y partes
            And la evaluación de origen se informa no disponible porque no existe una referencia persistida desde la solicitud, sin inferir procedencia manual o histórica
            And la dirección, propuestas, órdenes, reportes, reviews y hitos de pago se informan como ausentes
            And la cronología contiene únicamente eventos respaldados por datos persistidos y sus instantes se comparan como instantes exactos
            And no se inventa evento de contacto, aceptación, finalización, pago ni referencia faltante
            And la respuesta no contiene mensajes ni extractos de chat

    Rule: El acceso a medios requiere el permiso administrativo y nunca los expone públicamente
        Background:
            Given que existe la siguiente solicitud de trabajo:
                | solicitud | consumidor      | prestador        | creada               | estado  | título          | descripción      |
                | S1        | ana@example.com | juan@example.com | 2026-09-20T12:00:00Z | pending | Revisar cañería | Hay una pérdida. |
            And que "S1" tiene la siguiente imagen privada confirmada:
                | archivo   | nombre original | mime_type  | propósito          | creada               |
                | request-1 | caño.jpg        | image/jpeg | job_request_image | 2026-09-20T12:01:00Z |

        @wip
        Scenario: 64.1.6-OD Autorizar la recuperación de una imagen privada de la solicitud
            When solicito la imagen privada "request-1" como administrador de operaciones
            Then el sistema responde con estado 200 y el contenido corresponde al archivo "request-1"
            And la respuesta de imagen incluye "Cache-Control" con valor "private, no-store"
            And la respuesta no expone acceso público anónimo a la imagen

        @wip
        Scenario: 64.1.7-OD Rechazar la recuperación de una imagen privada sin autenticación administrativa
            Given que no envío un token Bearer
            When solicito la imagen privada "request-1" como administrador de operaciones
            Then el sistema responde con estado 401
            And no se entrega ningún byte de la imagen

        @wip
        Scenario: 64.1.7a-OD Rechazar la recuperación de una imagen privada sin permiso de operaciones
            Given que estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_audit"
            When solicito la imagen privada "request-1" como administrador de operaciones
            Then el sistema responde con estado 403
            And no se entrega ningún byte de la imagen

    Rule: Autenticación y permiso limitan la consulta de una solicitud existente
        Background:
            Given que existe la siguiente solicitud de trabajo sin referencia persistida a una evaluación de origen:
                | solicitud | consumidor      | prestador        | creada               | estado  | título               | descripción             |
                | S1        | ana@example.com | juan@example.com | 2026-09-20T12:00:00Z | pending | Revisar instalación | Hay una falla eléctrica. |

        Scenario Outline: 64.1.8-OD Rechazar una consulta sin autenticación válida
            Given que <estado de autenticación>
            When intento consultar el detalle administrativo de la operación de la solicitud "S1"
            Then el sistema responde con estado 401
            And la respuesta no contiene datos del detalle

            Examples:
                | estado de autenticación  |
                | no envío un token Bearer |
                | envío un token Bearer inválido |

        Scenario: 64.1.9-OD Rechazar a un administrador sin permiso de consulta de operaciones
            Given que estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_audit"
            When intento consultar el detalle administrativo de la operación de la solicitud "S1"
            Then el sistema responde con estado 403
            And la respuesta no contiene datos del detalle

    Rule: La identidad y validación limitan otras consultas

        Scenario: 64.1.10-OD Informar no encontrado para una operación inexistente
            Given que no existe ninguna solicitud con ID persistido 987654321 ni propuesta con ID persistido 987654321
            When consulto el detalle administrativo de la operación "jr-987654321"
            Then el sistema responde con estado 404
            And la respuesta no contiene datos del detalle

        Scenario Outline: 64.1.11-OD Rechazar identificadores de operación inválidos
            When consulto el detalle administrativo de la operación "<identificador>"
            Then el sistema responde con estado 400
            And la respuesta no contiene datos del detalle

            Examples:
                | identificador |
                | jr-0          |
                | sp-abc        |
                | xx-12         |
                | jr-1-extra    |

    Rule: La lectura se audita antes de entregar el detalle y falla cerrada sin modificar datos
        Background:
            Given que existe la siguiente solicitud de trabajo:
                | solicitud | consumidor      | prestador        | creada               | estado   | título          | descripción      |
                | S1        | ana@example.com | juan@example.com | 2026-09-20T12:00:00Z | accepted | Revisar cañería | Hay una pérdida. |

        @wip
        Scenario: 64.1.12-OD Preparar el resultado de auditoría antes de entregar el detalle sin cambiar la contratación
            Given que no existe un evento de auditoría para la correlación "request-operation-detail-1"
            When consulto el detalle administrativo de la solicitud "S1" con la correlación "request-operation-detail-1"
            Then el sistema responde con estado 200 y el detalle completo de "S1"
            And antes de entregar el detalle queda preparado exactamente un evento de acceso a "job_request" con el ID persistido de "S1", el operador "operador@example.com" y la correlación "request-operation-detail-1"
            And la solicitud "S1" y sus relaciones conservan exactamente los estados y los instantes previos a la consulta
            And la consulta no crea ni modifica propuestas, órdenes, pagos, conversaciones ni mensajes

        Scenario: 64.1.13-OD No entregar el detalle cuando falla el registro de auditoría
            Given que falla el almacenamiento del evento de auditoría de esta consulta
            When consulto el detalle administrativo de la solicitud "S1"
            Then el sistema responde con estado 500
            And la respuesta no contiene datos del detalle
            And no se entregan parcialmente los datos de "S1"
            And la solicitud "S1" y sus relaciones permanecen sin cambios
