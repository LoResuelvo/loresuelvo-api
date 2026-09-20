Feature: Listar rubros de prestadores
    Como usuario autenticado de LoResuelvo
    quiero consultar el catálogo de rubros
    para utilizar un rubro disponible en los flujos de la plataforma

    @wip
    Rule: Los usuarios autenticados pueden consultar el catálogo sin permisos administrativos

        Scenario: 38.1.8-LR Listar los rubros por nombre en orden ascendente
            Given que existen los siguientes rubros:
                | nombre       |
                | Plomería     |
                | Electricidad |
                | Albañilería  |
            And que estoy autenticado sin permisos administrativos
            When consulto el listado de rubros
            Then el sistema devuelve los siguientes rubros en este orden:
                | nombre       |
                | Albañilería  |
                | Electricidad |
                | Plomería     |

        Scenario: 38.1.9-LR Informar solamente los datos públicos de cada rubro
            Given que existe el rubro "Plomería"
            And que estoy autenticado sin permisos administrativos
            When consulto el listado de rubros
            Then cada rubro incluye solamente su identificador y su nombre visible
            And el listado no expone el nombre normalizado ni datos internos de persistencia

        Scenario: 38.1.10-LR Devolver un listado vacío
            Given que no existen rubros registrados
            And que estoy autenticado sin permisos administrativos
            When consulto el listado de rubros
            Then el sistema devuelve un listado de rubros vacío

    @wip
    Rule: Solo los usuarios autenticados pueden consultar el catálogo

        Scenario: 38.1.11-LR Rechazar el listado sin autenticación
            Given que no tengo una sesión válida
            When consulto el listado de rubros
            Then el sistema deniega el acceso
