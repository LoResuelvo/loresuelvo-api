Feature: Consultar el chat de una contratación con justificación auditada
    Como operador de soporte autorizado
    quiero consultar la conversación vinculada a una contratación después de justificar el acceso
    para investigar un incidente sin habilitar la lectura indiscriminada de mensajes privados

    Background:
        Given que existe el rubro "Plomería"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And que existe un administrador provisionado con correo "soporte@example.com", nombre "Sofía" y apellido "López"
        And que existe la siguiente solicitud de trabajo con una única conversación de trabajo "C1" creada junto con ella y activada al ser aceptada por "juan@example.com":
            | solicitud | consumidor      | prestador        | creada               | estado   | título          | descripción      |
            | S1        | ana@example.com | juan@example.com | 2026-09-20T12:00:00Z | accepted | Revisar cañería | Hay una pérdida. |
        And que estoy autenticado como administrador "soporte@example.com" con el permiso "read:admin_chat_audit"

    Rule: Sólo se entrega el chat de la conversación vinculada a la identidad estable de la operación

        Scenario: 64.2.1-CO Consultar mensajes con una justificación auditada
            Given que "C1" tiene los siguientes mensajes persistidos:
                | mensaje | remitente  | contenido                        | creado               |
                | M1      | consumer   | Hay una pérdida bajo la pileta.  | 2026-09-20T12:01:00Z |
                | M2      | provider   | ¿Podés enviarme una foto?        | 2026-09-20T12:02:00Z |
            When consulto el chat administrativo de la operación de la solicitud "S1" con la cabecera "X-Audit-Reason" igual a "Investigar el incidente 123" y la correlación "request-chat-1"
            Then el sistema responde con estado 200
            And la respuesta identifica la operación "jr-" seguida del ID persistido de "S1", la conversación "C1" y su vínculo con la solicitud
            And la página contiene "M1" y "M2" en ese orden, con ID, rol remitente, contenido e instante de creación persistidos
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"
            And antes de entregar los mensajes queda persistido un evento de acceso con operador "soporte@example.com", operación "S1", conversación "C1", motivo "Investigar el incidente 123", fecha, resultado y correlación "request-chat-1"
            And el resultado auditado indica la preparación de la entrega, no la recepción o lectura por el cliente
            And el evento de auditoría no contiene mensajes ni adjuntos del chat

        Scenario: 64.2.2-CO Identificar que varias propuestas comparten una conversación
            Given que existen las siguientes propuestas vinculadas a "S1":
                | propuesta | creada               | estado   |
                | P1        | 2026-09-20T13:00:00Z | accepted |
                | P2        | 2026-09-21T13:00:00Z | pending  |
            And que "C1" contiene los siguientes mensajes persistidos:
                | mensaje | remitente | contenido                   | creado               |
                | M1      | consumer  | Consulta sobre la primera.  | 2026-09-20T13:05:00Z |
                | M2      | consumer  | Consulta sobre la segunda.  | 2026-09-21T13:05:00Z |
            When consulto el chat administrativo de la operación "sp-" seguida del ID persistido de "P2" con motivo "Revisar intercambio sobre la segunda propuesta"
            Then el sistema responde con estado 200
            And la respuesta identifica la operación "sp-" seguida del ID persistido de "P2" y su vínculo con la conversación "C1" compartida con "P1"
            And la página incluye "M1" y "M2" con sus instantes persistidos, sin atribuir ambos exclusivamente a "P2"

        Scenario: 64.2.3-CO Consultar una conversación existente sin mensajes
            Given que "C1" no tiene mensajes
            When consulto el chat administrativo de la operación de la solicitud "S1" con motivo "Verificar conversación vacía"
            Then el sistema responde con estado 200
            And la página contiene una colección vacía, no nula, y no tiene cursor siguiente

        Scenario: 64.2.4-CO No permitir elegir otra conversación ni obtener recursos no relacionados
            Given que existe otra conversación de trabajo "C2" con mensajes privados que no está vinculada a "S1"
            When intento consultar el chat administrativo de la operación de la solicitud "S1" con motivo "Investigar la solicitud" y el parámetro de consulta "conversation_id" igual al ID persistido de "C2"
            Then el sistema responde con estado 400
            And la respuesta no contiene mensajes ni URLs de adjuntos de "C2"

    Rule: Cada página exige autorización y auditoría independientes y una justificación válida

        Scenario: 64.2.5-CO Limitar la primera página sin descargar el historial completo
            Given que "C1" contiene los siguientes mensajes persistidos en el orden indicado, con IDs crecientes:
                | mensaje | remitente | contenido        | creado               |
                | M1      | consumer  | Primera consulta. | 2026-09-20T12:01:00Z |
                | M2      | consumer  | Segunda consulta. | 2026-09-20T12:02:00Z |
                | M3      | consumer  | Tercera consulta. | 2026-09-20T12:02:00Z |
            When consulto el chat administrativo de la operación de la solicitud "S1" con límite 2, motivo "Revisar página inicial" y correlación "request-chat-page-1"
            Then el sistema responde con estado 200
            And la página contiene "M1" y "M2" en ese orden y entrega un cursor siguiente, sin incluir "M3"

        Scenario: 64.2.5a-CO Auditar nuevamente la página siguiente sin repetir mensajes
            Given que "C1" contiene los siguientes mensajes persistidos en el orden indicado, con IDs crecientes:
                | mensaje | remitente | contenido        | creado               |
                | M1      | consumer  | Primera consulta. | 2026-09-20T12:01:00Z |
                | M2      | consumer  | Segunda consulta. | 2026-09-20T12:02:00Z |
                | M3      | consumer  | Tercera consulta. | 2026-09-20T12:02:00Z |
            And que una consulta autorizada previa de "S1" con límite 2, motivo "Revisar página inicial" y correlación "request-chat-page-1" devolvió "M1" y "M2" y un cursor siguiente
            When consulto el chat administrativo de la operación de la solicitud "S1" con el cursor recibido, reutilizando el motivo "Revisar página inicial" en "X-Audit-Reason" y con correlación "request-chat-page-2"
            Then el sistema responde con estado 200
            And la página contiene sólo "M3" y no tiene cursor siguiente
            And quedan persistidos dos eventos de acceso distintos, ambos con el motivo "Revisar página inicial" reutilizado y con las correlaciones "request-chat-page-1" y "request-chat-page-2" respectivamente

        Scenario Outline: 64.2.6-CO Rechazar un motivo ausente, blanco o excesivo
            When intento consultar el chat administrativo de "S1" con <motivo>
            Then el sistema responde con estado 400
            And la respuesta no contiene mensajes ni URLs de adjuntos

            Examples:
                | motivo                              |
                | la cabecera X-Audit-Reason ausente |
                | la cabecera X-Audit-Reason en blanco |
                | una cabecera X-Audit-Reason que supera la longitud máxima documentada |

        Scenario Outline: 64.2.7-CO Rechazar límites y cursores inválidos para una operación existente
            When intento consultar el chat administrativo de la operación de la solicitud "S1" con motivo "Investigar incidente" y <parámetro inválido>
            Then el sistema responde con estado 400
            And la respuesta no contiene mensajes ni URLs de adjuntos

            Examples:
                | parámetro inválido                          |
                | límite 0                                   |
                | límite mayor que el máximo documentado    |
                | cursor "no-es-un-cursor"                    |

        Scenario Outline: 64.2.7a-CO Rechazar identificadores de operación inválidos
            When intento consultar el chat administrativo de la operación "<identificador>" con motivo "Investigar incidente"
            Then el sistema responde con estado 400
            And la respuesta no contiene mensajes ni URLs de adjuntos

            Examples:
                | identificador |
                | jr-0          |
                | sp-abc        |

    Rule: El permiso de soporte es independiente del permiso de operaciones y de los endpoints de participantes

        Scenario Outline: 64.2.8-CO Rechazar la consulta sin autenticación válida
            Given que <estado de autenticación>
            When intento consultar el chat administrativo de "S1" con motivo "Investigar incidente"
            Then el sistema responde con estado 401
            And la respuesta no contiene mensajes ni URLs de adjuntos

            Examples:
                | estado de autenticación       |
                | no envío un token Bearer      |
                | envío un token Bearer inválido |

        Scenario Outline: 64.2.9-CO Denegar la consulta inicial y la continuación sin permiso de chat
            Given que <contexto de consulta>
            And que estoy autenticado como administrador "soporte@example.com" solamente con el permiso "read:admin_operations"
            When intento consultar el chat administrativo de la operación de la solicitud "S1" <configuración del cursor> con motivo "Investigar incidente"
            Then el sistema responde con estado 403
            And la respuesta no contiene mensajes ni URLs de adjuntos

            Examples:
                | contexto de consulta                                                                                                                                                          | configuración del cursor |
                | no se realizó ninguna consulta previa de "S1"                                                                                                                                | sin cursor               |
                | "C1" tiene tres mensajes persistidos y una consulta autorizada previa de "S1" con límite 2 y motivo "Revisar página inicial" devolvió un cursor siguiente válido              | con el cursor recibido   |

        Scenario: 64.2.10-CO No ampliar el endpoint de conversación de los participantes
            Given que estoy autenticado como administrador "soporte@example.com" solamente con el permiso "read:admin_chat_audit"
            When intento consultar "C1" por el endpoint de detalle de conversación de participantes
            Then el sistema responde con estado 403
            And no se entregan mensajes ni URLs de adjuntos

        Scenario: 64.2.11-CO Informar no encontrado para una operación inexistente
            Given que no existe ninguna solicitud con ID persistido 987654321 ni propuesta con ID persistido 987654321
            When consulto el chat administrativo de la operación "jr-987654321" con motivo "Investigar incidente"
            Then el sistema responde con estado 404
            And la respuesta no contiene mensajes ni URLs de adjuntos

    Rule: Los adjuntos privados sólo se resuelven después de comprobar pertenencia y registrar el acceso

        Scenario: 64.2.12-CO Fallar cerrado si no se puede guardar la evidencia de acceso
            Given que "C1" tiene mensajes persistidos y un adjunto privado confirmado
            And que falla el almacenamiento del evento de auditoría de esta consulta
            When consulto el chat administrativo de la operación de la solicitud "S1" con motivo "Investigar incidente"
            Then el sistema responde con estado 500
            And la respuesta no contiene mensajes ni URLs de adjuntos
            And no se generó ningún acceso al adjunto

    Rule: La consulta de soporte no altera la conversación ni filtra datos privados por otras vías

        Scenario: 64.2.13-CO Conservar los mensajes y el estado de los participantes
            Given que "C1" tiene mensajes y un estado persistidos
            When consulto el chat administrativo de la operación de la solicitud "S1" con motivo "Investigar incidente"
            Then el sistema responde con estado 200
            And "C1" conserva sus mensajes y estado
            And no se crean mensajes, no se acepta ninguna solicitud y el operador no queda suscripto a canales de participantes
