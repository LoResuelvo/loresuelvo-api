Feature: US-16 - Ver trabajos realizados de prestador
    Como consumidor
    quiero visualizar trabajos realizados por un prestador
    para evaluar la calidad y experiencia del profesional antes de contratarlo

    Background:
        Given que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"

    Rule: El detalle siempre informa el resumen de reputación

        Scenario: 16.1.1-VPWH Informar promedio cero cuando el prestador no tiene reviews
            Given que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el detalle informa un promedio de rating de 0

        Scenario: 16.1.2-VPWH Informar cantidad cero cuando el prestador no tiene reviews
            Given que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el detalle informa una cantidad de ratings de 0

        Scenario: 16.1.3-VPWH Calcular el promedio de las reviews del prestador
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 5 estrellas
            And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "80000.00" para la fecha y hora "2026-08-15T16:00:00Z" con la descripción:
                """
                Reparación de pérdida en el baño.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 4 estrellas
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el detalle informa un promedio de rating de 4.5

        Scenario: 16.1.4-VPWH Contar las reviews del prestador
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 5 estrellas
            And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "80000.00" para la fecha y hora "2026-08-15T16:00:00Z" con la descripción:
                """
                Reparación de pérdida en el baño.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 4 estrellas
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el detalle informa una cantidad de ratings de 2

    Rule: El historial público contiene únicamente trabajos pagados

        Scenario: 16.2.1-VPWH Informar un historial vacío sin trabajos pagados
            Given que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el detalle de trabajos realizados es un arreglo vacío

        Scenario: 16.2.2-VPWH Mostrar un trabajo pagado aunque no tenga review
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el historial incluye un trabajo con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """

        Scenario: 16.2.3-VPWH Excluir una orden que todavía no fue pagada
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Trabajo pendiente de pago.
                """
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el historial no incluye un trabajo con la descripción:
                """
                Trabajo pendiente de pago.
                """

        Scenario: 16.2.4-VPWH Ordenar los trabajos desde el más reciente
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Trabajo anterior.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "80000.00" para la fecha y hora "2026-08-16T15:00:00Z" con la descripción:
                """
                Trabajo más reciente.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el primer trabajo del historial tiene la descripción:
                """
                Trabajo más reciente.
                """

        Scenario: 16.2.5-VPWH Mostrar el reporte de finalización del trabajo pagado
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el trabajo del historial incluye el reporte de finalización:
                """
                Trabajo finalizado y funcionamiento verificado.
                """

    Rule: El historial puede incluir la review asociada al trabajo pagado

        Scenario: 16.3.1-VPWH Mostrar el rating de la review del trabajo
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 5 estrellas
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el trabajo del historial incluye una review de 5 estrellas

        Scenario: 16.3.2-VPWH Mostrar el comentario de la review del trabajo
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 5 estrellas con la descripción "Trabajo prolijo y excelente atención."
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el trabajo del historial incluye el comentario de review "Trabajo prolijo y excelente atención."

    Rule: El historial público no expone datos privados del trabajo

        Scenario: 16.4.1-VPWH No exponer la identidad del consumidor en el historial
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el historial no expone la identidad del consumidor "Ana Pérez"

        Scenario: 16.4.2-VPWH No exponer el importe en el historial
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el historial no expone el importe "100000.00"

        Scenario: 16.4.3-VPWH No exponer las imágenes de evidencia en el historial
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle público del prestador "juan.plomero@example.com"
            Then el historial no expone imágenes de evidencia
