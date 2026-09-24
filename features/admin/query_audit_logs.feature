Feature: Consultar el registro de auditoría administrativa
    Como responsable de auditoría autorizado de LoResuelvo
    quiero consultar y filtrar las acciones administrativas registradas
    para investigar quién actuó, sobre qué recurso, cuándo, con qué resultado y con qué justificación

    Background:
        Given que existe un administrador provisionado con correo "supervisor@example.com", nombre "Sofía" y apellido "López"

    Rule: La consulta autorizada expone únicamente eventos y datos permitidos

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:admin_audit"

        Scenario: 63.1-AL Consultar eventos existentes sin exponer datos privados
            Given que existen los siguientes eventos de auditoría preexistentes:
                | evento | operador              | acción  | tipo de recurso | ID de recurso | fecha UTC            | resultado | correlación | motivo             |
                | A      | supervisor@example.com | create  | category        | 17            | 2026-09-20T10:00:00Z | succeeded | request-a   |                    |
                | B      | supervisor@example.com | execute | payment         | 42            | 2026-09-21T10:00:00Z | succeeded | request-b   | Revisión aprobada  |
            When consulto el registro de auditoría administrativa para el operador "supervisor@example.com"
            Then el sistema responde con estado 200
            And la página contiene los eventos "B" y "A" en ese orden
            And cada evento expone exactamente su identificador, referencia interna del operador, acción, tipo e ID de recurso, fecha UTC, resultado, correlación y motivo sólo cuando existe
            And el evento "B" informa el motivo "Revisión aprobada" y el evento "A" no informa motivo
            And la respuesta no expone credenciales, identificadores externos de autenticación, cuerpos HTTP, mensajes de chat, biometría ni URLs firmadas
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"
            And los eventos "A" y "B" permanecen sin modificaciones

        Scenario: 63.2-AL Devolver una colección vacía sin confundirla con un fallo
            Given que existe un administrador provisionado con correo "auditor@example.com", nombre "Ana" y apellido "Pérez"
            And que existe un evento de auditoría preexistente del operador "auditor@example.com"
            When consulto el registro de auditoría administrativa para el operador "supervisor@example.com"
            Then el sistema responde con estado 200
            And la página contiene una colección vacía, no nula, y no tiene cursor siguiente
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

    Rule: Los filtros se combinan y la ventana temporal es explícita

        Background:
            Given que existe un administrador provisionado con correo "auditor@example.com", nombre "Ana" y apellido "Pérez"
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:admin_audit"

        Scenario: 63.3-AL Filtrar por operador, acción, recurso, resultado y rango temporal mediante AND
            Given que existen los siguientes eventos de auditoría preexistentes:
                | evento | operador              | acción  | tipo de recurso | ID de recurso | fecha UTC            | resultado | correlación |
                | A      | supervisor@example.com | execute | payment         | 42            | 2026-09-20T10:00:00Z | succeeded | request-a   |
                | B      | auditor@example.com    | execute | payment         | 42            | 2026-09-20T10:00:00Z | succeeded | request-b   |
                | C      | supervisor@example.com | create  | payment         | 42            | 2026-09-20T10:00:00Z | succeeded | request-c   |
                | D      | supervisor@example.com | execute | category        | 42            | 2026-09-20T10:00:00Z | succeeded | request-d   |
                | E      | supervisor@example.com | execute | payment         | 43            | 2026-09-20T10:00:00Z | succeeded | request-e   |
                | F      | supervisor@example.com | execute | payment         | 42            | 2026-09-20T10:00:00Z | failed    | request-f   |
                | G      | supervisor@example.com | execute | payment         | 42            | 2026-09-21T00:00:00Z | succeeded | request-g   |
            When filtro el registro por el operador "supervisor@example.com", la acción "execute", el recurso "payment" con ID "42", el resultado "succeeded" y el rango desde "2026-09-20T10:00:00Z" hasta "2026-09-21T00:00:00Z"
            Then el sistema responde con estado 200
            And la página contiene solamente el evento "A"
            And el inicio del rango es inclusivo y el fin es exclusivo

    Rule: La paginación conserva un corte estable y un orden determinista

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:admin_audit"

        @wip
        Scenario: 63.4-AL Recorrer páginas sin duplicados ni eventos posteriores al corte
            Given que existen los siguientes eventos de auditoría preexistentes:
                | evento | operador               | acción  | recurso  | ID de recurso | fecha UTC            | resultado | correlación | orden de ID |
                | A      | supervisor@example.com | execute | category | 17            | 2026-09-20T12:00:00Z | succeeded | request-a   | 5           |
                | B      | supervisor@example.com | execute | category | 17            | 2026-09-20T11:00:00Z | succeeded | request-b   | 4           |
                | X      | supervisor@example.com | create  | category | 17            | 2026-09-20T11:00:00Z | succeeded | request-x   | 3           |
                | C      | supervisor@example.com | execute | category | 17            | 2026-09-20T11:00:00Z | succeeded | request-c   | 2           |
                | D      | supervisor@example.com | execute | category | 17            | 2026-09-20T10:00:00Z | succeeded | request-d   | 1           |
            And que se agregará un evento sintético del operador "supervisor@example.com" con acción "execute", recurso "category" e ID "17", resultado "succeeded" y correlación única después de la primera página
            When recorro el registro para el operador "supervisor@example.com" y la acción "execute" con páginas de 2 eventos
            Then la primera página contiene los eventos "A" y "B" en ese orden y entrega un cursor siguiente
            And la segunda página contiene los eventos "C" y "D" en ese orden y no tiene cursor siguiente
            And ningún evento se repite ni aparecen el evento "X" o el evento posterior al corte

        @wip
        Scenario Outline: 63.5-AL Aplicar el límite predeterminado y el máximo documentados
            Given que existen 101 eventos sintéticos preexistentes del operador "supervisor@example.com" con acción "create", recurso "category" e ID "17", resultado "succeeded", fechas anteriores a la consulta e identificadores y correlaciones únicos
            When consulto el registro para el operador "supervisor@example.com" <configuración del límite>
            Then la página contiene <cantidad> eventos y entrega un cursor siguiente

            Examples:
                | configuración del límite | cantidad |
                | sin indicar límite       | 20       |
                | con límite 100           | 100      |

    Rule: La autenticación, el permiso y los parámetros restringen la consulta

        Scenario Outline: 63.6-AL Rechazar la consulta sin autenticación válida
            Given que existe un evento de auditoría preexistente
            And que <estado de autenticación>
            When intento consultar el registro de auditoría administrativa
            Then el sistema responde con estado 401
            And la respuesta no contiene eventos de auditoría

            Examples:
                | estado de autenticación                  |
                | no envío un token Bearer                 |
                | envío un token Bearer inválido          |

        Scenario: 63.7-AL Rechazar a un administrador sin el permiso de auditoría
            Given que existe un evento de auditoría preexistente
            And que estoy autenticado como administrador "supervisor@example.com" solamente con el permiso "read:providers"
            When intento consultar el registro de auditoría administrativa
            Then el sistema responde con estado 403
            And la respuesta no contiene eventos de auditoría

        @wip
        Scenario Outline: 63.8-AL Rechazar filtros y límites inválidos
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:admin_audit"
            When consulto el registro de auditoría con el parámetro "<parámetro>" igual a "<valor>"
            Then el sistema responde con estado 400
            And la respuesta no contiene eventos de auditoría

            Examples:
                | parámetro     | valor                |
                | operator_id   | 0                    |
                | action        | unknown              |
                | resource_type | Payment              |
                | resource_id   | https://private.test |
                | result        | unknown              |
                | occurred_from | no-es-fecha          |
                | occurred_to   | no-es-fecha          |
                | limit         | 0                    |
                | limit         | 101                  |

        Scenario: 63.9-AL Rechazar un rango temporal invertido
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:admin_audit"
            When consulto el registro desde "2026-09-21T00:00:00Z" hasta "2026-09-20T00:00:00Z"
            Then el sistema responde con estado 400
            And la respuesta no contiene eventos de auditoría

        @wip
        Scenario Outline: 63.10-AL Rechazar un cursor ilegible o incompatible con los filtros
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:admin_audit"
            And que existen 3 eventos de auditoría preexistentes del operador "supervisor@example.com" con acción "create"
            And que obtuve un cursor válido al filtrar el registro por el operador "supervisor@example.com" y la acción "create" con límite 1
            When consulto el registro con <cursor y filtros>
            Then el sistema responde con estado 400
            And la respuesta no contiene eventos de auditoría

            Examples:
                | cursor y filtros                                                     |
                | el cursor alterado, el mismo operador y la acción "create"           |
                | ese cursor, el mismo operador y la acción "execute"                  |

    Rule: El acceso al propio registro se audita antes de entregar datos y falla de forma controlada

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:admin_audit"

        Scenario: 63.11-AL Registrar una sola evidencia de acceso sin recursión ni motivo manual
            Given que existe un administrador provisionado con correo "auditor@example.com", nombre "Ana" y apellido "Pérez"
            And que existe un evento de auditoría preexistente del operador "auditor@example.com"
            When consulto el registro de auditoría administrativa para el operador "supervisor@example.com" con la correlación "request-audit-access"
            Then el sistema responde con estado 200 y una colección vacía no nula
            And queda registrado exactamente un evento de acceso preparado a la colección de auditoría por "supervisor@example.com"
            And ese evento contiene la correlación "request-audit-access" y no requiere motivo manual
            And el evento de esta consulta no aparece en la colección devuelta
