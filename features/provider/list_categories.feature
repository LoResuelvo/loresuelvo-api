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
