Feature: Consultar reclamos propios
    Como consumidor o prestador que presentó un reclamo
    quiero consultar mis expedientes y su resultado disponible
    para seguir su estado sin acceder a información de otras personas

    Background:
        Given que existen el consumidor registrado "ana@example.com" y el prestador registrado "juan@example.com"
        And que ambos participan en la solicitud de trabajo "S1"

    Rule: El listado propio devuelve únicamente los expedientes del usuario autenticado

        Scenario Outline: 31.13-CC Paginar reclamos propios con fechas deterministas y desempate descendente
            Given que el reloj del sistema indica "2026-09-25T12:00:00Z"
            And que "ana@example.com" tiene los reclamos abiertos "C1" y "C2" creados ambos el "2026-09-24T10:00:00Z", con el ID persistido de "C2" mayor que el de "C1", y "C3" creado el "2026-09-23T10:00:00Z", en operaciones distintas
            And que "ana@example.com" tiene el reclamo resuelto "C4" creado el "2026-09-25T11:00:00Z"
            And que "juan@example.com" tiene un reclamo propio más reciente creado el "2026-09-25T10:00:00Z"
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis reclamos abiertos en la página <pagina> con límite 2
            Then el sistema responde con estado 200 y la colección contiene exactamente "<reclamos>" en ese orden
            And la respuesta informa la página solicitada <pagina> y el límite 2, y no contiene reclamos de "juan@example.com" ni el reclamo resuelto "C4"
            And la respuesta incluye "Cache-Control: private, no-store"

            Examples:
                | pagina | reclamos |
                | 1      | C2, C1   |
                | 2      | C3       |
                | 3      | ninguno  |

        Scenario: 31.14-CC Devolver una página vacía como colección vacía
            Given que "ana@example.com" no tiene reclamos en estado "dismissed"
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto mis reclamos con estado "dismissed", página 1 y límite 20
            Then el sistema responde con estado 200 y una colección vacía
            And la respuesta no incluye reclamos de "juan@example.com"

    Rule: El detalle, resultado y evidencias respetan la propiedad del reclamante

        Scenario Outline: 31.15-CC No revelar si un reclamo propio existe para otro participante
            Given que el reclamo "<reclamo>" <existencia>
            And que estoy autenticado como prestador "juan@example.com"
            When consulto el detalle propio del reclamo "<reclamo>"
            Then el sistema responde con estado 404 sin incluir testimonio, operación ni evidencias
            And la respuesta incluye "Cache-Control: private, no-store"

            Examples:
                | reclamo | existencia                                       |
                | C1      | pertenece a "ana@example.com" sobre "S1"         |
                | C404    | no existe                                  |

        Scenario: 31.16-CC Devolver resolución vacía antes de un dictamen formal
            Given que "ana@example.com" presentó el reclamo "C1" y no existe resolución registrada en el contrato compartido con US-68
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle propio del reclamo "C1"
            Then el detalle informa la resolución como vacía

        Scenario: 31.17-CC Exponer sólo el resultado formal del expediente propio
            Given que "ana@example.com" presentó el reclamo finalizado "C1" con una resolución formal persistida por el contrato compartido con US-68
            And que el dictamen de "C1" tiene tipo semántico "a favor del consumidor", fundamentación "Se verificó el incumplimiento acordado" y fecha "2026-09-24T12:00:00Z"
            And que el dictamen sugiere una compensación de 1500 centavos ARS, sin afirmar que se ejecutó
            And que "juan@example.com" tiene un expediente separado "C2" con la evidencia privada "IMG2", no vinculada a "C1"
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle propio del reclamo "C1"
            Then el detalle presenta exactamente el tipo, fundamentación y fecha registrados para "C1"
            And informa exactamente la compensación sugerida de 1500 centavos ARS, sin afirmar que se ejecutó
            And la respuesta no expone la identidad privada del operador, auditorías de acceso, evidencias ajenas ni transacciones financieras completas
            And la respuesta incluye "Cache-Control: private, no-store"

        Scenario: 31.18-CC Incluir evidencia privada propia con acceso temporal en el detalle
            Given que "ana@example.com" tiene el reclamo "C1" con la imagen privada confirmada "IMG1" vinculada
            And que estoy autenticado como consumidor "ana@example.com"
            When consulto el detalle propio del reclamo "C1"
            Then el sistema responde con estado 200
            And el detalle incluye la imagen "IMG1" con una URL temporal privada
            And la respuesta no expone claves de almacenamiento ni credenciales

        Scenario: 31.19-CC No revelar el detalle ni las evidencias de otro participante
            Given que "ana@example.com" tiene el reclamo "C1" con la imagen privada confirmada "IMG1" vinculada
            And que estoy autenticado como prestador "juan@example.com"
            When consulto el detalle propio del reclamo "C1"
            Then el sistema responde con estado 404 sin incluir testimonio, operación ni evidencias
            And la respuesta no entrega una URL temporal

        Scenario: 31.19a-CC No incluir evidencia vinculada a otro reclamo en el detalle propio
            Given que "ana@example.com" tiene el reclamo "C1" y "juan@example.com" tiene el reclamo "C2" en la misma operación
            And que la imagen privada confirmada "IMG1" está vinculada a "C1" y no a "C2"
            And que estoy autenticado como prestador "juan@example.com"
            When consulto el detalle propio del reclamo "C2"
            Then el sistema responde con estado 200
            And el detalle no incluye la imagen "IMG1" ni una URL temporal para ella
