Feature: Notificar solicitudes de trabajo en tiempo real y en el teléfono
    Como prestador
    quiero recibir un aviso cuando un consumidor me envía una solicitud de trabajo
    para enterarme de nuevas oportunidades de trabajo

    Background:
        Given que Ana es una consumidora y Juan es un prestador registrados
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"

    @wip
    Scenario: 39.1.1 El prestador destinatario recibe la notificación con la información de la solicitud
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé las imágenes de solicitud de trabajo: "perdida-bajo-mesada.jpg", "detalle-sifon.webp", "humedad-pared.png"
        And que el consumidor "ana@example.com" está disponible para recibir mensajes en tiempo real
        And que el prestador "juan.plomero@example.com" está disponible para recibir mensajes en tiempo real
        And que el prestador "pedro.plomero@example.com" está disponible para recibir mensajes en tiempo real
        When envío una solicitud de trabajo al prestador "Juan Perez" con el título "Reparación de fuga en la cocina" y las imágenes cargadas:
            """
            Hola Juan, necesito reparar una fuga de agua en la cocina. ¿Podrías ayudarme esta semana?
            """
        Then el prestador "juan.plomero@example.com" recibe en tiempo real la notificación de una nueva solicitud de trabajo
        And la notificación identifica la solicitud de trabajo enviada
        And la notificación incluye los datos del consumidor, el título y la descripción de la solicitud
        And la notificación incluye las imágenes adjuntas a la solicitud
        And el consumidor "ana@example.com" no recibe la notificación de su propia solicitud de trabajo
        And el prestador "pedro.plomero@example.com" no recibe la notificación de esa solicitud de trabajo

    @wip
    Scenario: 39.1.2 El prestador recibe en su teléfono un aviso de nueva solicitud
        Given que estoy autenticado como consumidor "ana@example.com"
        And que cargué y confirmé la imagen de solicitud de trabajo "fuga-cocina.jpg"
        And que Juan tiene un teléfono registrado para recibir avisos
        And que Juan no está usando la aplicación
        When envío una solicitud de trabajo al prestador "Juan Perez" con el título "Reparación de fuga en la cocina" y la imagen cargada "fuga-cocina.jpg":
            """
            Hola Juan, necesito reparar una fuga de agua en la cocina.
            """
        Then el sistema envía solamente a Juan un aviso de nueva solicitud de trabajo
        And el aviso indica cuál es la solicitud recibida
        And el aviso no incluye los datos del consumidor, el título, la descripción ni enlaces a las imágenes adjuntas
