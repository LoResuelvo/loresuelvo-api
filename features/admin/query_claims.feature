Feature: Consultar la bandeja administrativa de reclamos
    Como operador de soporte y mediación de LoResuelvo
    quiero consultar los expedientes que requieren atención
    para identificar su estado y contexto sin acceder a contenido innecesario

    Background:
        Given que existen el consumidor registrado "ana@example.com" y el prestador registrado "juan@example.com"
        And que ambos participan en la solicitud de trabajo "S1"
        And que existe un administrador provisionado con correo "operador@example.com", nombre "Sofía" y apellido "López"

    Rule: La bandeja filtra y pagina expedientes sin duplicar sus relaciones

        Scenario Outline: 68.1-QC Ordenar y paginar reclamos por fecha real e ID persistido
            Given que la fecha y hora actual del sistema es "2026-09-25T12:00:00-03:00"
            And "ana@example.com" tiene los reclamos abiertos "C1", "C2" y "C3" reportados respectivamente el "2026-09-24T10:00:00-03:00", "2026-09-24T10:00:00-03:00" y "2026-09-23T10:00:00-03:00", con "C1", "C2" y "C3" vinculados a las operaciones distintas "S1", "S2" y "S3", y con el ID persistido de "C2" mayor que el de "C1"
            And "C1" tiene dos evidencias vinculadas
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto la bandeja administrativa de reclamos en la página <pagina> con límite 2 y estado "open"
            Then el sistema responde con estado 200 y la colección contiene exactamente "<reclamos>" en ese orden
            And cada reclamo aparece una sola vez e informa ID, fecha de reporte, tipo de reclamante, referencia de operación, rubro disponible y estado
            And el total de coincidencias es exactamente 3
            And la antigüedad de las filas de la página es exactamente "<antigüedades>" desde el reloj observado, sin SLA ni prioridad
            And la respuesta informa la página solicitada <pagina> y el límite 2
            And la consulta no registra eventos persistentes de auditoría
            And la respuesta incluye la cabecera "Cache-Control" con valor "private, no-store"

            Examples:
                | pagina | reclamos | antigüedades                 |
                | 1      | C2, C1   | C2: 26 horas, C1: 26 horas   |
                | 2      | C3       | C3: 50 horas                 |
                | 3      | ninguno  | ninguna                      |

        Scenario Outline: 68.2-QC Buscar por correo del reclamante o identidad canónica de operación
            Given "ana@example.com" tiene el reclamo "C1" sobre la operación de la solicitud "S1"
            And que existen dos propuestas de servicio de la conversación de "S1": "P1" es la primera y "P2" es posterior
            And "juan@example.com" tiene el reclamo "C2" sobre la operación de la propuesta posterior "P2" de "S1"
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When busco reclamos por "<consulta>" en la bandeja administrativa
            Then el sistema responde con estado 200 y contiene exactamente los reclamos "<reclamos>"
            And cada resultado conserva la identidad canónica de su operación y la referencia específica persistida

            Examples:
                | consulta                  | reclamos |
                | ana@example.com           | C1       |
                | juan@example.com          | C2       |
                | jr- seguido del ID de S1  | C1       |
                | sp- seguido del ID de P2  | C2       |

        Scenario Outline: 68.3-QC Filtrar por cada estado válido sin incluir otros estados
            Given que "ana@example.com" tiene un reclamo por cada estado del expediente: open, in_review, resolved y dismissed, vinculados respectivamente a las operaciones distintas "S1", "S2", "S3" y "S4"
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto la bandeja administrativa de reclamos con estado "<estado>"
            Then el sistema responde con estado 200 y contiene únicamente reclamos en estado "<estado>"

            Examples:
                | estado    |
                | open      |
                | in_review |
                | resolved  |
                | dismissed |

        Scenario: 68.4-QC Devolver una colección vacía cuando ningún reclamo coincide
            Given que "ana@example.com" tiene el reclamo "C1" en estado "open" sobre "S1"
            And que "juan@example.com" tiene el reclamo "C2" en estado "dismissed" sobre "S1"
            And estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto la bandeja administrativa de reclamos con búsqueda "ana@example.com" y estado "dismissed"
            Then el sistema responde con estado 200 y una colección vacía, no nula

        Scenario Outline: 68.5-QC Rechazar filtros y límites de paginación inválidos
            Given que estoy autenticado como administrador "operador@example.com" con el permiso "read:admin_claims"
            When consulto la bandeja administrativa de reclamos con <entrada>
            Then el sistema responde con estado 400 sin datos de reclamos

            Examples:
                | entrada                              |
                | el estado "pending"                 |
                | la página 0                          |
                | el límite 0                          |
                | el límite 101                        |

        Scenario: 68.6-QC Exigir el permiso de lectura de reclamos
            Given que estoy autenticado como administrador "operador@example.com" solamente con el permiso "read:admin_audit"
            When consulto la bandeja administrativa de reclamos
            Then el sistema responde con estado 403
            And la respuesta no contiene datos de expedientes
