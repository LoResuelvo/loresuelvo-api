Feature: Reseñar una orden de trabajo pagada
    Como consumidor
    quiero calificar el trabajo terminado
    para dejar constancia de mi experiencia con el prestador

    Background:
        Given que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"
        And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """

    Rule: Sólo el consumidor puede reseñar una orden pagada

        Scenario: 30.1.1-CRWO El consumidor reseña una orden pagada con descripción
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When creo una reseña para la orden con 5 estrellas y la descripción:
                """
                Trabajo prolijo y excelente atención.
                """
            Then el sistema registra la reseña con 5 estrellas
            And la reseña registrada tiene la descripción:
                """
                Trabajo prolijo y excelente atención.
                """

        Scenario: 30.1.2-CRWO El consumidor puede reseñar sólo con estrellas
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When creo una reseña para la orden con 4 estrellas y sin descripción
            Then el sistema registra la reseña con 4 estrellas
            And la reseña registrada no tiene descripción

        Scenario Outline: 30.1.3-CRWO Rechazar un rating inválido
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When intento crear una reseña para la orden con <rating> estrellas y la descripción "Trabajo correcto"
            Then el sistema rechaza la reseña con estado 400

            Examples:
                | rating |
                | 0      |
                | 6      |

        Scenario: 30.1.4-CRWO Rechazar una descripción demasiado extensa
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como consumidor "ana@example.com"
            When intento crear una reseña para la orden con una descripción de más de 500 caracteres y 5 estrellas
            Then el sistema rechaza la reseña con estado 400

        Scenario Outline: 30.1.5-CRWO Rechazar la reseña solicitada por <actor>
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que estoy autenticado como <rol> "<correo>"
            When intento crear una reseña para la orden con 5 estrellas y la descripción "Trabajo correcto"
            Then el sistema rechaza la reseña con estado 403

            Examples:
                | actor             | rol         | correo                    |
                | el prestador      | prestador   | juan.plomero@example.com  |
                | otro consumidor   | consumidor  | carla@example.com         |

    Rule: Sólo una orden pagada puede tener una reseña y sólo una vez

        Scenario Outline: 30.2.1-CRWO Rechazar reseñar una orden no pagada en estado <estado>
            Given que existe una orden de trabajo para reseña en estado "<estado>" para "ana@example.com" y "juan.plomero@example.com"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento crear una reseña para la orden con 5 estrellas y la descripción "Trabajo correcto"
            Then el sistema rechaza la reseña con estado 409

            Examples:
                | estado           |
                | scheduled        |
                | awaiting_payment |

        Scenario: 30.2.2-CRWO Rechazar una segunda reseña
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 5 estrellas
            And que estoy autenticado como consumidor "ana@example.com"
            When intento crear una reseña para la orden con 3 estrellas y la descripción "Otra opinión"
            Then el sistema rechaza la reseña con estado 409

    Rule: La reseña se visualiza en el detalle de la orden

        Scenario: 30.3.1-CRWO El consumidor consulta la reseña de su orden pagada
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 5 estrellas con la descripción "Trabajo prolijo"
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle de la orden de trabajo
            Then el sistema responde con estado 200
            And el detalle incluye la reseña de 5 estrellas
            And el detalle incluye la descripción de la reseña "Trabajo prolijo"

        Scenario: 30.3.2-CRWO El prestador consulta la reseña de su orden pagada
            Given que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
            And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo
            And que la orden ya tiene una reseña de 4 estrellas con la descripción "Trabajo correcto"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            When consulto el detalle de la orden de trabajo
            Then el sistema responde con estado 200
            And el detalle incluye la reseña de 4 estrellas
            And el detalle incluye la descripción de la reseña "Trabajo correcto"
