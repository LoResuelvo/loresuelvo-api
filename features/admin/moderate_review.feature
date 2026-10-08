Feature: Moderar la visibilidad de una reseña
    Como operador de soporte y moderación de LoResuelvo
    quiero decidir la visibilidad de una reseña y atender reportes identificados
    para retirar contenido inapropiado sin borrar el original ni alterar su valoración

    Background:
        Given que existe un consumidor registrado con correo "ana@example.com"
        And existe un prestador registrado con correo "juan@example.com"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: Cada decisión identifica su motivo y los reportes que atiende

        @wip
        Scenario: 69.6-MR Ocultar una reseña atendiendo su reporte pendiente
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00Z"
            And que la orden pagada "O1", programada más de 24 horas después de aceptar su propuesta, tiene una reseña visible de 5 estrellas con comentario original
            And que "juan@example.com", prestador calificado en "O1", reporta la reseña mientras está visible y su único reporte pendiente es "R1"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            When oculto la reseña de la orden "O1" con la categoría "personal_data", el motivo "El comentario publica un dato personal" y la versión esperada 1, atendiendo sólo el reporte "R1"
            Then el sistema responde con estado 200 y la visibilidad de la reseña pasa a "hidden"
            And se conserva el comentario original y queda registrada una decisión nueva vinculada a la reseña
            And la decisión registra el motivo, el operador autenticado y el instante actual del servidor
            And queda confirmado exactamente un evento asociado a esa decisión y a la reseña "O1", con operador, acción, instante real del servidor y correlación, sin copiar el original ni el motivo
            And "R1" queda atendido como procedente
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        @wip
        Scenario Outline: 69.7-MR Ocultar directamente una reseña sin reportes según la categoría
            Given que la orden pagada "O1" tiene una reseña visible de 3 estrellas y no tiene reportes
            And que estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            When oculto la reseña de la orden "O1" con la categoría "<categoria>", el motivo "<motivo>" y la versión esperada 1 sin seleccionar reportes
            Then el sistema responde con estado 200 y la reseña pasa a visibilidad "hidden"
            And el comentario original se conserva y queda registrada la decisión sin reportes atendidos

            Examples:
                | categoria          | motivo                                    |
                | abusive_language   | El comentario contiene lenguaje abusivo  |
                | personal_data      | El comentario publica un dato personal    |
                | spam_advertising   | El comentario incluye publicidad ajena    |
                | unrelated_content  | El comentario no trata sobre el servicio  |

        @wip
        Scenario: 69.8-MR Desestimar reportes pendientes sin cambiar la visibilidad
            Given que la orden pagada "O1" tiene una reseña visible de 4 estrellas
            And que "juan@example.com", prestador calificado en "O1", reportó la reseña mientras estaba visible y su único reporte pendiente es "R1"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            When desestimo el reporte "R1" de la reseña "O1" con el motivo "El contenido describe el servicio recibido" y la versión esperada 1
            Then el sistema responde con estado 200 y la reseña permanece visible
            And "R1" queda atendido como no procedente
            And queda registrada una decisión nueva sin ocultar la reseña ni modificar su calificación

        @wip
        Scenario: 69.9-MR Restablecer la visibilidad conservando la decisión de ocultación revisada
            Given que la orden pagada "O1" tiene una reseña visible en la versión 1
            And que "juan@example.com", prestador calificado en "O1", reportó la reseña mientras estaba visible y el reporte pendiente "R1" existe
            And que estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            And que oculté "O1" con categoría "personal_data", motivo "El comentario publica un dato personal" y versión esperada 1, atendiendo "R1" como procedente en la decisión "D1"
            When restablezco la visibilidad de la reseña "O1" con el motivo "La revisión confirma que el texto puede mostrarse" y la versión esperada 2
            Then el sistema responde con estado 200 y la reseña "O1" queda visible en la versión 3
            And queda registrada la decisión nueva "D2" vinculada a la decisión de ocultación "D1"
            And el historial conserva intacta "D1" y "R1" sigue atendido como procedente

        @wip
        Scenario: 69.10-MR Rechazar un reintento de ocultación cuya versión quedó desactualizada
            Given que la orden pagada "O1" tiene una reseña visible de 5 estrellas en la versión 1
            And que estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            And que oculté la reseña "O1" con categoría "spam_advertising", motivo "Publicidad ajena al servicio" y versión esperada 1, pero se perdió la respuesta
            And que el mismo operador restableció "O1" con motivo "La revisión confirma que el texto puede mostrarse" y versión esperada 2
            When repito la solicitud original de ocultación de "O1" con categoría "spam_advertising", motivo "Publicidad ajena al servicio" y versión esperada 1
            Then el sistema responde con estado 409
            And la reseña permanece visible en la versión 3, con dos decisiones y dos eventos históricos, sin una nueva decisión ni otro evento

        @wip
        Scenario Outline: 69.11-MR Rechazar entradas inválidas de una solicitud de moderación
            Given que la orden pagada "O1" tiene una reseña visible en la versión 1 y el único reporte pendiente "R1" de "juan@example.com", su prestador calificado
            And que para la entrada indicada el resto de la solicitud es válido y estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            When intento moderar la reseña "O1" con <entrada>
            Then el sistema responde con estado 400 y no registra una decisión ni cambia la reseña ni los reportes

            Examples:
                | entrada                                                                   |
                | la categoría de ocultación "repetitive_reporting"                         |
                | el motivo vacío al ocultar con categoría "personal_data" y versión 1       |
                | el motivo que excede 500 bytes UTF-8 al ocultar con categoría "personal_data" y versión 1 |
                | desestimar sin identificar reportes, con versión 1                           |

        @wip
        Scenario: 69.12-MR Denegar una decisión sin permiso de escritura de moderación
            Given que la orden pagada "O1" tiene una reseña visible
            And que estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_reviews"
            When intento ocultar la reseña "O1" con la categoría "abusive_language", el motivo "El comentario contiene lenguaje abusivo" y la versión esperada 1
            Then el sistema responde con estado 403
            And la reseña permanece visible y no se registra una decisión

        @wip
        Scenario: 69.13-MR No atender automáticamente un reporte recibido tras cargar la reseña
            Given que la orden pagada "O1" tiene una reseña visible en la versión 1 y aún no tiene reportes
            And que el administrador "operador@example.com" consulta la reseña antes de que exista un reporte
            And que "juan@example.com", prestador calificado en "O1", reporta la reseña mientras sigue visible y su único reporte "R1" queda pendiente
            And que estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            And que oculté "O1" sin seleccionar reportes con categoría "personal_data", motivo "El comentario publica un dato personal" y versión esperada 1
            When oculto nuevamente la reseña "O1", ya oculta, atendiendo "R1" con categoría "personal_data", motivo "Se confirma la divulgación reportada" y versión esperada 2
            Then el sistema responde con estado 200 y "O1" permanece oculta en la versión 3
            And "R1" queda atendido como procedente por esta nueva decisión
            And el historial conserva las dos decisiones y el evento asociado a cada una
