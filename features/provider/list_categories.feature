Feature: Listar rubros de prestador
    Como prestador
    quiero consultar los rubros disponibles
    para registrarme con un rubro definido

    Rule: Se deben listar los rubros disponibles para el registro

        Scenario: 01-LR Listar rubros disponibles correctamente
            Given que existe el rubro "Plomería"
            And que existe el rubro "Electricidad"
            When consulto el listado de rubros
            Then el sistema muestra los rubros disponibles
            And el listado incluye el rubro "Plomería"
            And el listado incluye el rubro "Electricidad"

        Scenario: 02-LR Listar rubros cuando no hay rubros registrados
            Given que no existen rubros registrados
            When consulto el listado de rubros
            Then el sistema muestra un listado de rubros vacío

    @wip
    Rule: El listado debe respetar el contrato público del catálogo

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

    @wip
    Rule: Solo los usuarios autenticados pueden consultar el catálogo

        Scenario: 38.1.10-LR Rechazar el listado sin autenticación
            Given que no tengo una sesión válida
            When consulto el listado de rubros
            Then el sistema deniega el acceso
