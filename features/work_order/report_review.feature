Feature: Reportar una reseña para revisión administrativa
    Como prestador titular de la orden que recibió una reseña
    quiero reportar una reseña visible y accesible
    para solicitar su revisión sin modificar la valoración del servicio

    Background:
        Given que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que la cuenta de Mercado Pago "mp-juan" está vinculada al prestador "juan.plomero@example.com"
        And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-17T15:00:00Z" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """
        And que el prestador "juan.plomero@example.com" informó la finalización con evidencia válida de la orden
        And que el pago aprobado del saldo dejó la orden de trabajo pagada por completo

    Rule: El prestador titular puede reportar una reseña visible de su orden

        Background:
            Given que la orden ya tiene una reseña de 5 estrellas con la descripción "Trabajo prolijo y excelente atención."

        @wip
        Scenario Outline: 30.2.1-RR Registrar un reporte con una categoría admitida
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When reporto la reseña de la orden con categoría "<categoria>" y sin explicación
            Then el sistema responde con estado 201
            And la respuesta incluye el identificador y el estado pendiente del reporte
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"
            And la respuesta no expone el texto original de la reseña "Trabajo prolijo y excelente atención."

            Examples:
                | categoria          |
                | abusive_language   |
                | personal_data      |
                | spam_advertising   |
                | unrelated_content  |

        @wip
        Scenario Outline: 30.2.2-RR Aceptar una explicación vacía o compuesta solo por espacios
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When reporto la reseña de la orden con categoría "spam_advertising" y explicación <forma>
            Then el sistema responde con estado 201
            And la respuesta incluye el identificador y el estado pendiente del reporte

            Examples:
                | forma                        |
                | vacía                        |
                | compuesta solo por espacios  |

        @wip
        Scenario Outline: 30.2.3-RR Validar el límite de caracteres Unicode de la explicación
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When reporto la reseña de la orden con categoría "unrelated_content" y una explicación de <cantidad> caracteres
            Then el sistema responde con estado <estado>

            Examples:
                | cantidad | estado |
                | 500      | 201    |
                | 501      | 400    |

        @wip
        Scenario: 30.2.4-RR Rechazar un reporte sin categoría
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When intento reportar la reseña de la orden sin especificar categoría
            Then el sistema responde con estado 400

        @wip
        Scenario: 30.2.5-RR Rechazar un reporte con una categoría no admitida
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            When intento reportar la reseña de la orden con categoría no admitida "other"
            Then el sistema responde con estado 400

    Rule: Una reseña admite un solo reporte durante toda su vida

        Background:
            Given que la orden ya tiene una reseña de 5 estrellas con la descripción "Trabajo prolijo y excelente atención."

        @wip
        Scenario: 30.2.6-RR Rechazar un segundo reporte de la misma reseña
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And reporté la reseña de la orden con categoría "personal_data" y explicación "El comentario contiene datos personales del prestador."
            And el ingreso de ese reporte respondió con estado 201
            When intento reportar la reseña de la orden con categoría "abusive_language"
            Then el sistema responde con estado 409
            And la reseña conserva un solo reporte

    Rule: Sólo el prestador titular de la orden puede reportar su reseña

        Background:
            Given que la orden ya tiene una reseña de 5 estrellas con la descripción "Trabajo prolijo y excelente atención."

        @wip
        Scenario Outline: 30.2.7-RR Rechazar el reporte de una reseña por <actor>
            Given que están habilitadas las zonas de cobertura "Comuna 6"
            And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 6"
            And que estoy autenticado como <rol> "<correo>"
            When intento reportar la reseña de la orden con categoría "personal_data"
            Then el sistema responde con estado 403

            Examples:
                | actor                 | rol        | correo                    |
                | la consumidora autora | consumidor | ana@example.com           |
                | otro prestador       | prestador  | pedro.plomero@example.com |
