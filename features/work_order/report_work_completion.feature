@wip
Feature: Informar finalización de una orden de trabajo
    Como prestador asignado
    quiero informar que realicé el trabajo adjuntando una descripción y fotografías
    para dejar evidencia y habilitar al consumidor a pagar el saldo

    Background:
        Given que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
        And que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que existe un consumidor registrado con correo "carla@example.com", nombre "Carla" y apellido "Gómez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And que existe una orden de trabajo programada para la propuesta aceptada de "juan.plomero@example.com" para "ana@example.com" por "100000.00" para la fecha y hora "2026-08-15T15:00:00Z" con la descripción:
            """
            Reparación de pérdida de agua en cocina con materiales incluidos.
            """

    Rule: Sólo el prestador asignado puede informar la finalización

        Scenario: 26.1.1-IFWO El prestador asignado informa la finalización con una foto
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que cargué y confirmé una imagen privada de finalización "trabajo.jpg" para la orden
            When informo la finalización de la orden con la imagen "trabajo.jpg" y la descripción:
                """
                Trabajo finalizado y funcionamiento verificado.
                """
            Then el sistema registra el reporte de finalización
            And la orden de trabajo queda en estado "awaiting_payment"

        Scenario: 26.1.2-IFWO El prestador asignado informa la finalización con tres fotos
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que cargué y confirmé tres imágenes privadas de finalización: "antes.jpg", "durante.png" y "después.webp"
            When informo la finalización de la orden con las imágenes "antes.jpg", "durante.png" y "después.webp" y la descripción:
                """
                Trabajo terminado, probado y entregado al consumidor.
                """
            Then el sistema registra el reporte de finalización con las tres imágenes en ese orden

        Scenario Outline: 26.1.3-IFWO Rechazar el reporte de finalización solicitado por <actor>
            Given que estoy autenticado como <rol> "<correo>"
            And que cargué y confirmé una imagen privada de finalización "trabajo.jpg" para la orden
            When intento informar la finalización de la orden con la imagen "trabajo.jpg" y la descripción:
                """
                Trabajo finalizado.
                """
            Then el sistema rechaza el reporte de finalización con estado 403

            Examples:
                | actor              | rol         | correo                     |
                | el consumidor      | consumidor  | ana@example.com            |
                | otro consumidor    | consumidor  | carla@example.com          |
                | otro prestador     | prestador   | pedro.plomero@example.com  |

    Rule: La finalización sólo puede informarse después del turno

        Scenario: 26.2.1-IFWO Rechazar el reporte antes de la fecha programada
            Given que la fecha y hora actual del sistema es "2026-08-15T14:59:59Z"
            And que estoy autenticado como prestador "juan.plomero@example.com"
            And que cargué y confirmé una imagen privada de finalización "trabajo.jpg" para la orden
            When intento informar la finalización de la orden con la imagen "trabajo.jpg" y la descripción:
                """
                Trabajo finalizado antes del turno.
                """
            Then el sistema rechaza el reporte de finalización con estado 409

    Rule: El reporte de finalización es único y válido

        Scenario: 26.3.1-IFWO Rechazar un segundo reporte de finalización
            Given que la orden ya tiene un reporte de finalización válido
            And que estoy autenticado como prestador "juan.plomero@example.com"
            And que cargué y confirmé una imagen privada de finalización "segunda.jpg" para la orden
            When intento informar nuevamente la finalización de la orden con la imagen "segunda.jpg" y la descripción:
                """
                Segundo reporte.
                """
            Then el sistema rechaza el reporte de finalización con estado 409

        Scenario: 26.3.2-IFWO Rechazar una descripción vacía o con espacios
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que cargué y confirmé una imagen privada de finalización "trabajo.jpg" para la orden
            When intento informar la finalización de la orden con la imagen "trabajo.jpg" y la descripción:
                """
                   
                """
            Then el sistema rechaza el reporte de finalización con estado 400

        Scenario Outline: 26.3.3-IFWO Rechazar una cantidad inválida de imágenes
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que preparé <cantidad> imágenes para el reporte de finalización
            When intento informar la finalización de la orden con la descripción "Trabajo finalizado"
            Then el sistema rechaza el reporte de finalización con estado 400

            Examples:
                | cantidad |
                | cero     |
                | cuatro   |

    Rule: Sólo se aceptan imágenes privadas confirmadas y aptas para finalización

        Scenario Outline: 26.4.1-IFWO Rechazar una imagen no disponible para finalización
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que preparé una imagen "trabajo.jpg" <condición> para el reporte de finalización
            When intento informar la finalización de la orden con la descripción "Trabajo finalizado"
            Then el sistema rechaza el reporte de finalización con estado 400

            Examples:
                | condición                         |
                | perteneciente a otro prestador    |
                | pendiente de confirmación         |
                | con propósito incorrecto          |
                | con formato no permitido          |
                | que supera los 5 MB                |

    Rule: La notificación se persiste y se entrega después del commit

        Scenario: 26.5.1-IFWO Notificar al consumidor la finalización registrada
            Given que estoy autenticado como prestador "juan.plomero@example.com"
            And que el consumidor "ana@example.com" está disponible para recibir mensajes en tiempo real
            And que cargué y confirmé una imagen privada de finalización "trabajo.jpg" para la orden
            When informo la finalización de la orden con la descripción "Trabajo finalizado" y la imagen "trabajo.jpg"
            Then el sistema registra la notificación de finalización para el consumidor "ana@example.com"
            And el consumidor "ana@example.com" recibe en tiempo real la notificación de finalización
