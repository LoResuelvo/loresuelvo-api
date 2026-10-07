Feature: Presentar un reclamo sobre una operación propia
    Como consumidor o prestador autenticado
    quiero presentar un reclamo sobre una operación en la que participo
    para dejar constancia de lo ocurrido y solicitar la intervención de soporte

    Background:
        Given que existen el consumidor registrado "ana@example.com" y el prestador registrado "juan@example.com"
        And que ambos participan en la solicitud de trabajo "S1"

    Rule: El ingreso resuelve al reclamante y la operación desde datos persistidos

        Scenario Outline: 31.1-IC Presentar un reclamo propio antes de que exista una orden o un pago
            Given que estoy autenticado como <parte> "<correo>"
            And que la solicitud "S1" no tiene propuesta, orden ni intento de pago
            When presento un reclamo por incumplimiento sobre la solicitud "S1" con la clave "550e8400-e29b-41d4-a716-446655440000" y el testimonio "El servicio acordado no se realizó"
            Then el sistema responde con estado 201 y la ubicación del reclamo creado
            And el acuse identifica la operación "jr-" seguida del ID persistido de "S1", la referencia "job_request_id" con el ID persistido de "S1" y el estado "open"
            And el expediente registra como reclamante "<correo>" y como tipo de parte "<tipo>"
            And el expediente conserva el testimonio presentado y no inventa una orden ni evidencia adjunta

            Examples:
                | parte      | correo           | tipo     |
                | consumidor | ana@example.com  | consumer |
                | prestador  | juan@example.com | provider |

        Scenario: 31.2-IC Resolver cada propuesta posterior como una operación independiente
            Given que existen dos propuestas de servicio de la conversación de "S1": "P1" es la primera y "P2" es posterior
            And que estoy autenticado como prestador "juan@example.com"
            When presento un reclamo por mala calidad sobre la propuesta "P2" con la clave "550e8400-e29b-41d4-a716-446655440002" y el testimonio "La propuesta posterior tuvo un problema"
            Then el expediente queda vinculado a "sp-" seguido del ID persistido de "P2" y conserva la referencia "service_proposal_id" con el ID persistido de "P2"
            And el expediente no se atribuye a "jr-" seguido del ID persistido de "S1" por compartir conversación o participantes

        Scenario: 31.3-IC Evitar un segundo reclamo abierto propio aunque se use otra referencia de la misma operación
            Given que "ana@example.com" ya presentó el reclamo "C1" sobre "S1" usando la solicitud como referencia y el expediente sigue "open"
            And que "P1" es la primera propuesta de la conversación de "S1"
            And que estoy autenticado como consumidor "ana@example.com"
            When presento otro reclamo sobre la propuesta "P1" con una clave nueva y un testimonio diferente
            Then el sistema responde con estado 409 sin revelar información de reclamos ajenos
            And el reclamo "C1" conserva su testimonio, estado y evidencia sin duplicar actuaciones

        Scenario: 31.4-IC Permitir que la contraparte presente su propio expediente en la misma operación
            Given que "ana@example.com" presentó el reclamo "C1" sobre "S1" con la clave "550e8400-e29b-41d4-a716-446655440004", motivo "improper_charge" y testimonio "El cobro solicitado no corresponde"
            And que estoy autenticado como prestador "juan@example.com"
            When presento el mismo reclamo sobre la solicitud "S1" con la misma clave "550e8400-e29b-41d4-a716-446655440004" y el mismo contenido
            Then el sistema responde con estado 201 y crea un expediente propio distinto de "C1" para "juan@example.com"
            And la respuesta no identifica ni devuelve el expediente "C1" de "ana@example.com"

        Scenario: 31.5-IC Adjuntar imágenes privadas confirmadas a una presentación
            Given que "ana@example.com" cargó y confirmó la imagen JPEG válida "IMG1" y la imagen PNG válida "IMG2" con finalidad "claim_evidence_image"
            And que ambas imágenes son privadas y de hasta 5 MiB cada una
            And que estoy autenticado como consumidor "ana@example.com"
            When presento un reclamo sobre la solicitud "S1" con esas imágenes y el testimonio "Adjunto evidencia del incumplimiento"
            Then el expediente conserva los IDs estables "IMG1" y "IMG2" como evidencias vinculadas
            And la evidencia no se representa mediante una URL firmada ni se expone públicamente

        Scenario Outline: 31.6-IC Rechazar referencias de operación ajenas, inexistentes o intentos de pago privados
            Given que la solicitud "S2" pertenece a otra persona, "S404" no existe y "P1" es una propuesta de "S1" con el intento de pago "PI1"
            And que el prestador no está autorizado a consultar "PI1" por las reglas del módulo de pagos
            And que estoy autenticado como prestador "juan@example.com"
            And que el request es válido salvo por la autorización o existencia de la referencia
            When intento iniciar un reclamo con la referencia "<referencia>"
            Then el sistema responde con estado 404 sin crear un expediente ni revelar datos de la referencia

            Examples:
                | referencia                                               |
                | solicitud ajena "S2"                                     |
                | solicitud inexistente "S404"                             |
                | intento de pago privado "PI1" de su propia operación     |

        Scenario Outline: 31.7-IC Rechazar una credencial ausente, inválida o sin cuenta local habilitada
            Given que la credencial de autenticación es "<credencial>"
            When intento presentar un reclamo sobre la solicitud "S1"
            Then el sistema responde con estado <estado> sin crear un expediente

            Examples:
                | credencial                 | estado |
                | ausente                     | 401    |
                | JWT inválido                | 401    |
                | identidad sin cuenta local  | 403    |

    Rule: La clave de idempotencia identifica una presentación sin duplicar su expediente

        Scenario Outline: 31.8-IC Recuperar el expediente actual al reintentar una presentación confirmada
            Given que "ana@example.com" presentó el reclamo "C1" con la clave "550e8400-e29b-41d4-a716-446655440001" y el contenido original está asociado a esa clave
            And que el estado actual del reclamo "C1" es "<estado>"
            And que estoy autenticado como consumidor "ana@example.com"
            When repito el ingreso de "C1" con la misma clave y el mismo contenido
            Then el sistema responde con estado 200 y el mismo identificador "C1" con su estado actual "<estado>"
            And el expediente, sus actuaciones y sus vínculos de evidencia no se duplican

            Examples:
                | estado    |
                | open      |
                | in_review |
                | resolved  |
                | dismissed |

        Scenario: 31.9-IC Rechazar la reutilización de una clave con contenido diferente
            Given que "ana@example.com" presentó el reclamo "C1" con la clave "550e8400-e29b-41d4-a716-446655440003"
            And que estoy autenticado como consumidor "ana@example.com"
            When intento presentar contenido distinto usando la clave ya asociada a "C1"
            Then el sistema responde con estado 409 sin crear ni modificar un expediente

        Scenario: 31.10-IC Presentar un nuevo reclamo después de finalizar el anterior
            Given que "ana@example.com" tiene el reclamo finalizado "C1" con su dictamen formal persistido por el contrato compartido con US-68
            And que estoy autenticado como consumidor "ana@example.com"
            When presento un nuevo reclamo sobre la misma operación con una clave nueva
            Then el sistema responde con estado 201 y crea un expediente distinto "C2" en estado "open"
            And "C1" conserva su identidad, estado final y dictamen sin reapertura ni modificación
