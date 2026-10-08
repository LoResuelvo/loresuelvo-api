Feature: Consultar la bandeja administrativa de reseñas
    Como operador de soporte y moderación de LoResuelvo
    quiero consultar reseñas según su visibilidad y reportes pendientes
    para priorizar revisiones sin exponer contenido restringido en la bandeja

    Background:
        Given que existe un consumidor registrado con correo "ana@example.com"
        And existe un prestador registrado con correo "juan@example.com"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: La bandeja separa visibilidad de atención de reportes y resume cada reseña una sola vez

        Scenario: 69.1-QR Devolver una bandeja vacía cuando no hay reportes pendientes
            Given que la orden pagada "O1" tiene una reseña visible y no tiene reportes pendientes
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_reviews"
            When consulto la bandeja administrativa de reseñas con estado "reported"
            Then el sistema responde con estado 200 y una colección vacía, no nula
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

        Scenario Outline: 69.2-QR Filtrar reseñas con el único reporte posible por reseña
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00Z"
            And que las órdenes "O1", "O2" y "O3" corresponden a servicios completados y pagados, programados más de 24 horas después de aceptar sus propuestas y con fecha ya transcurrida, con reseñas elegibles
            And que las reseñas "O1" y "O2" están visibles y la reseña "O3" está oculta
            And que "juan@example.com", prestador calificado en "O1", reporta su reseña mientras está visible y su único reporte "R1" queda pendiente con fecha del servidor "2026-09-25T12:00:00Z"
            And que "O2" y "O3" no tienen reportes pendientes
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_reviews"
            When consulto la bandeja administrativa de reseñas con estado "<estado>"
            Then el sistema responde con estado 200 y contiene exactamente el conjunto de reseñas "<ordenes>", con una sola fila por reseña
            And cada fila informa orden, consumidor, prestador, operación, calificación, visibilidad y cantidad de reportes pendientes "<cantidades>"
            And las filas con reporte pendiente indican "2026-09-25T12:00:00Z" como fecha y las demás no tienen fecha de reporte
            And la bandeja no incluye comentarios originales, explicaciones de reportes ni justificaciones administrativas
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

            Examples:
                | estado   | ordenes   | cantidades |
                | reported | O1        | O1=1       |
                | visible  | O1, O2    | O1=1, O2=0 |
                | hidden   | O3        | O3=0       |
                | all      | O1, O2, O3| O1=1, O2=0, O3=0 |

        Scenario Outline: 69.3-QR Paginar por identidad estable de la orden
            Given que existen tres órdenes pagadas creadas en este orden: "O1", "O2" y "O3", cada una con una reseña visible, y se captura para cada etiqueta el identificador generado por el sistema
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_reviews"
            When consulto la bandeja administrativa de reseñas en la página <pagina> con límite 2 y estado "visible"
            Then el sistema responde con estado 200 y entrega exactamente las reseñas "<etiquetas>" en ese orden
            And la página devuelve las reseñas ordenadas por identificador generado descendente, sin repetirlas entre páginas

            Examples:
                | pagina | etiquetas |
                | 1      | O3, O2    |
                | 2      | O1        |
