Feature: Consultar el detalle administrativo de una reseña
    Como operador autorizado de soporte y moderación
    quiero consultar el original y el contexto acotado de sus reportes y decisiones
    para evaluar una reseña sin revelar contenido restringido a otros accesos

    Background:
        Given que existe un consumidor registrado con correo "ana@example.com"
        And existe un prestador registrado con correo "juan@example.com"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: El detalle conserva la identidad de la orden y limita el contexto sensible al permiso de moderación

        Scenario: 69.4-GRD Mostrar el reporte único y el historial acotado de decisiones
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00Z"
            And que la orden pagada "O1", programada más de 24 horas después de aceptar su propuesta, tiene una reseña visible de 5 estrellas con el comentario "Quedó impecable y limpio"
            And que "juan@example.com", prestador calificado en "O1", reporta la reseña mientras está visible y su único reporte pendiente es "R1"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "write:admin_reviews"
            And que oculté la reseña "O1" atendiendo "R1" con categoría "personal_data", motivo "El comentario publica un dato personal" y versión esperada 1
            And que restablecí la visibilidad de "O1" con motivo "La revisión confirma que el texto puede mostrarse" y versión esperada 2
            And que volví a ocultar "O1" sin atender reportes con categoría "personal_data", motivo "Se mantiene la restricción del comentario" y versión esperada 3
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_reviews"
            When consulto el detalle administrativo de la reseña de la orden "O1" con decisiones página 1 límite 1
            Then el sistema responde con estado 200 y el detalle de la reseña identificada por "O1"
            And el detalle informa el comentario original "Quedó impecable y limpio", 5 estrellas, visibilidad "hidden", consumidor, prestador, orden y operación canónica
            And la reseña está en la versión 4 y conserva el único reporte "R1" y tres decisiones en su historial
            And "R1" permanece atendido como procedente por la primera decisión
            And la página solicitada contiene exactamente una decisión de un total de 3 y señala que hay otra página
            And el detalle presenta explicaciones y motivos sólo en este acceso de moderación
            And se confirma exactamente un evento de acceso asociado a la reseña "O1" con el operador, la acción, el instante real del servidor y la correlación, sin copiar el original, las explicaciones ni los motivos
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

    Rule: El rol administrativo por sí solo no permite recuperar el original

        Scenario: 69.5-GRD Rechazar la lectura del detalle sin el permiso de moderación
            Given que la orden pagada "O1" tiene una reseña oculta con comentario original
            And que estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_operations"
            When consulto el detalle administrativo de la reseña de la orden "O1"
            Then el sistema responde con estado 403
            And la respuesta no contiene el comentario original, explicaciones ni motivos de moderación
