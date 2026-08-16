Feature: Consultar detalle y evidencia de una orden de trabajo
    Como participante de una orden de trabajo
    quiero consultar su detalle y la evidencia presentada
    para verificar el trabajo antes del pago y conservar su historial

    Background:
        Given que la fecha y hora actual del sistema es "2026-07-04T10:00:00-03:00"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    Rule: Los participantes pueden consultar el detalle contractual de la orden

        Scenario: 27.1.1-GWOD Consumidor consulta una orden programada sin reporte
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle de la orden de trabajo
            Then el sistema responde con estado 200
            And el detalle informa el estado "scheduled"
            And el detalle incluye el importe "100000.00"
            And el detalle incluye la fecha de servicio "2026-08-15T15:00:00Z"
            And el detalle incluye la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And el detalle no incluye un reporte de finalización

        Scenario: 27.1.2-GWOD Prestador consulta una orden programada sin reporte
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina con materiales incluidos.
                """
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When consulto el detalle de la orden de trabajo
            Then el sistema responde con estado 200
            And el detalle informa el estado "scheduled"
            And el detalle incluye los datos contractuales de la propuesta aceptada
            And el detalle no incluye un reporte de finalización

    Rule: El detalle conserva la evidencia presentada antes y después del pago

        Scenario: 27.2.1-GWOD Consumidor consulta una orden pendiente de pago con evidencia
            Given que existe una orden de trabajo en estado "awaiting_payment" para "ana@example.com" y "juan.plomero@example.com"
            And que la orden tiene el reporte de finalización con la descripción:
                """
                Trabajo finalizado y funcionamiento verificado.
                """
            And que el reporte tiene las imágenes privadas "trabajo.jpg" y "detalle.png" en ese orden
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle de la orden de trabajo
            Then el sistema responde con estado 200
            And el detalle informa el estado "awaiting_payment"
            And el detalle incluye la descripción del reporte:
                """
                Trabajo finalizado y funcionamiento verificado.
                """
            And el detalle incluye la fecha del reporte
            And el detalle incluye las imágenes "trabajo.jpg" y "detalle.png" en ese orden
            And cada imagen incluye una URL temporal privada

        Scenario: 27.2.2-GWOD Prestador consulta una orden pagada con evidencia
            Given que existe una orden de trabajo en estado "paid" para "ana@example.com" y "juan.plomero@example.com"
            And que la orden tiene el reporte de finalización con la descripción:
                """
                Trabajo finalizado y funcionamiento verificado.
                """
            And que el reporte tiene la imagen privada "trabajo.jpg"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When consulto el detalle de la orden de trabajo
            Then el sistema responde con estado 200
            And el detalle informa el estado "paid"
            And el detalle incluye la descripción del reporte:
                """
                Trabajo finalizado y funcionamiento verificado.
                """
            And el detalle incluye la fecha del reporte y la fecha de pago
            And el detalle incluye la imagen "trabajo.jpg" con una URL temporal privada

    Rule: Sólo los participantes pueden consultar el detalle

        Scenario Outline: 27.3.1-GWOD Rechazar el detalle solicitado por un usuario ajeno
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com"
            And que estoy autenticado como <rol> "<correo>"
            When consulto el detalle de la orden de trabajo
            Then el sistema responde con estado 403

            Examples:
                | rol         | correo                    |
                | consumidor  | carla@example.com         |
                | prestador   | pedro.plomero@example.com |

        Scenario: 27.3.2-GWOD Devolver 404 para una orden inexistente
            Given que estoy autenticado como consumidor "ana@example.com"
            And que no existe una orden de trabajo con identificador "999999"
            When consulto el detalle de la orden de trabajo inexistente
            Then el sistema responde con estado 404
