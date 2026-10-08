Feature: Respetar la visibilidad moderada de una reseña
    Como usuario de LoResuelvo
    quiero que las consultas muestren reseñas según su visibilidad
    para ocultar el contenido moderado sin alterar la valoración ni la contratación

    Background:
        Given que existe un consumidor registrado con correo "ana@example.com"
        And existe un prestador registrado con correo "juan@example.com"

    Rule: Las consultas públicas ocultan el original moderado y conservan el trabajo y los agregados

        Scenario: 69.14-RV Excluir el comentario oculto del perfil público y su historial de trabajos
            Given que "juan@example.com" tiene dos trabajos pagados reseñados: "O1" con 5 estrellas, oculta y con el comentario "Quedó impecable y limpio", y "O2" con 3 estrellas visibles
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el perfil público del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And el perfil incluye el trabajo pagado "O1" en su historial y no incluye el comentario "Quedó impecable y limpio"
            And el perfil informa dos calificaciones registradas y un promedio de 4,00

        Scenario: 69.15-RV Mantener la calificación oculta en los indicadores de reputación
            Given que "juan@example.com" tiene dos trabajos pagados con reseñas: una reseña oculta de 5 estrellas con el comentario "Quedó impecable y limpio" y una reseña visible de 3 estrellas
            And que estoy autenticado como prestador "juan@example.com"
            When consulto mi reputación
            Then veo dos calificaciones registradas, una reseña visible y un promedio de 4,00
            And la distribución conserva una calificación de 5 estrellas y una de 3 estrellas
            And la respuesta de reputación no incluye el comentario oculto "Quedó impecable y limpio"

        Scenario: 69.16-RV Informar que la orden ya tiene reseña aunque esté oculta
            Given que la orden pagada "O1", programada más de 24 horas después de aceptar su propuesta, ya tiene una reseña oculta de 5 estrellas
            And que estoy autenticado como consumidor "ana@example.com"
            When intento crear otra reseña para la orden "O1" con 3 estrellas y el comentario "Otra opinión"
            Then el sistema rechaza la reseña con estado 409

    Rule: Las consultas administrativas ordinarias tampoco recuperan el original oculto

        Scenario: 69.17-RV Omitir el comentario oculto del detalle administrativo de operación
            Given que la orden pagada "O1" tiene una reseña oculta de 5 estrellas con el comentario "Quedó impecable y limpio"
            And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_operations"
            When consulto el detalle administrativo de la operación de la orden "O1"
            Then el sistema responde con estado 200
            And el detalle administrativo conserva las 5 estrellas y no incluye el comentario original

        Scenario: 69.18-RV Omitir el comentario oculto del diagnóstico administrativo del prestador
            Given que la orden pagada "O1" tiene una reseña oculta de 5 estrellas con el comentario "Quedó impecable y limpio"
            And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"
            And que estoy autenticado como administrador "operador@example.com" con el permiso "read:providers"
            When consulto el diagnóstico administrativo del prestador "juan@example.com"
            Then el sistema responde con estado 200
            And el diagnóstico informa una calificación registrada y un promedio de 5,00, sin incluir el comentario original de "O1"

    Rule: Las consultas de participantes conservan la calificación sin revelar el original oculto

        Scenario Outline: 69.19-RV Ocultar el comentario moderado en el detalle propio de la orden
            Given que la orden pagada "O1" pertenece a "ana@example.com" como consumidor y a "juan@example.com" como prestador, y tiene una reseña oculta de 5 estrellas con el comentario "Quedó impecable y limpio"
            And que estoy autenticado como <rol> "<correo>"
            When consulto el detalle propio de la orden "O1"
            Then el sistema responde con estado 200
            And el detalle conserva las 5 estrellas y no incluye el comentario original "Quedó impecable y limpio"

            Examples:
                | rol        | correo           |
                | consumidor | ana@example.com  |
                | prestador  | juan@example.com |
