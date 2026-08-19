Feature: US-30.1 - Ver reputación en la búsqueda de prestadores
    Como consumidor
    quiero conocer la reputación de cada prestador en los resultados de búsqueda
    para comparar profesionales antes de contactarlos

    Background:
        Given que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Pérez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"
        And que la cuenta de Mercado Pago "mp-pedro" está vinculada al prestador "pedro.plomero@example.com"

    Rule: Cada resultado informa el resumen de reputación del prestador

        Scenario: 30.1.1-RSP Informar resumen cero para un prestador sin reviews
            When filtro técnicos por el rubro "Plomería"
            Then el resultado del prestador "Juan Pérez" informa un promedio de rating de 0 y una cantidad de ratings de 0

        Scenario: 30.1.2-RSP Informar promedio y cantidad de reviews del prestador
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
            When filtro técnicos por el rubro "Plomería"
            Then el resultado del prestador "Juan Pérez" informa un promedio de rating de 4.5 y una cantidad de ratings de 2

    Rule: El resumen se asocia con el prestador correcto
    
        Scenario: 30.1.3-RSP Asociar el rating de cada prestador a su resultado
            Given que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
                """
                Reparación de pérdida de agua en cocina.
                """
            And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 5 estrellas
            And que existe una orden de trabajo programada para la propuesta aceptada de "pedro.plomero@example.com" para "ana@example.com" por "80000.00" para la fecha y hora "2026-08-15T16:00:00Z" con la descripción:
                """
                Reparación de pérdida en el baño.
                """
            And que el prestador "pedro.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 2 estrellas
            When filtro técnicos por el rubro "Plomería"
            Then el resultado del prestador "Juan Pérez" informa un promedio de rating de 5 y una cantidad de ratings de 1
            And el resultado del prestador "Pedro Dib" informa un promedio de rating de 2 y una cantidad de ratings de 1
