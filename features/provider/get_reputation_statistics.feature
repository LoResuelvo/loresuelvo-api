Feature: Consultar mi reputación y la cobertura de reseñas
    Como prestador de LoResuelvo
    quiero conocer cómo valoran mis trabajos y cuántos trabajos pagados tienen una reseña
    para entender la calificación recibida y la cobertura de esa valoración

    Background:
        Given que existe el consumidor "ana@example.com"
        And que existen los prestadores "juan@example.com" y "pedro@example.com"
        And que estoy autenticado como prestador "juan@example.com"

    Rule: Mis indicadores resumen todas las reseñas de mis trabajos pagados

        @wip
        Scenario: 72.1-RP Calcular el promedio y la distribución de todas mis calificaciones
            Given que tengo cinco trabajos pagados, con reseñas de 1, 2, 3, 4 y 5 estrellas
            When consulto mi reputación
            Then veo 5 trabajos pagados elegibles y 5 con reseña
            And veo 5 reseñas, un promedio de 3,00 y una cobertura del 100,00 %
            And la distribución muestra una reseña para cada calificación de 1 a 5 estrellas

        @wip
        Scenario: 72.2-RP Incluir las calificaciones sin reseñas en la distribución
            Given que tengo un trabajo pagado con una reseña de 5 estrellas
            When consulto mi reputación
            Then la distribución incluye las cinco calificaciones de 1 a 5 estrellas
            And la calificación de 5 estrellas tiene una reseña y las demás tienen cero
            And la suma de la distribución coincide con la cantidad total de reseñas

        @wip
        Scenario: 72.3-RP Contar como elegibles solamente mis trabajos pagados
            Given que tengo dos trabajos pagados y uno todavía sin pagar
            And que uno de mis trabajos pagados tiene una reseña de 4 estrellas
            When consulto mi reputación
            Then veo 2 trabajos pagados elegibles, 1 con reseña, una cobertura del 50,00 % y un promedio de 4,00
            And la reseña del trabajo pagado aporta al promedio y no se cuenta el trabajo sin pagar

        @wip
        Scenario: 72.4-RP Mostrar cero de cobertura cuando mis trabajos pagados aún no tienen reseñas
            Given que tengo dos trabajos pagados sin reseña
            When consulto mi reputación
            Then veo 2 trabajos pagados elegibles, 0 con reseña y 0 reseñas
            And la cobertura es 0,00 %, el promedio no tiene valor y la página de reseñas está vacía
            And no hay otra página de reseñas para consultar

        @wip
        Scenario: 72.5-RP Mostrar indicadores vacíos cuando todavía no tuve trabajos pagados
            Given que no tengo trabajos pagados
            When consulto mi reputación
            Then veo cero trabajos pagados elegibles, cero trabajos con reseña y cero reseñas
            And las cinco calificaciones tienen cero reseñas
            And el promedio y la cobertura no tienen valor, la página está vacía y no hay otra página para consultar

        @wip
        Scenario: 72.6-RP Contar una reseña aunque su descripción esté vacía
            Given que tengo un trabajo pagado con una reseña de 4 estrellas y descripción vacía
            When consulto mi reputación y sus reseñas
            Then veo la reseña con su calificación y descripción vacía
            And esa reseña cuenta para el promedio, la distribución y la cobertura
            And no se inventa una descripción para completar la reseña

        @wip
        Scenario: 72.7-RP Redondear el promedio cuando queda justo a mitad de un centésimo
            Given que tengo 32 trabajos pagados, de los cuales 8 tienen reseña
            And las ocho calificaciones son una de 1 estrella y siete de 4 estrellas
            When consulto mi reputación
            Then veo un promedio de 3,63 y una cobertura del 25,00 %

        @wip
        Scenario: 72.8-RP Redondear la cobertura cuando queda justo a mitad de un centésimo
            Given que tengo 32 trabajos pagados y uno tiene una reseña de 5 estrellas
            When consulto mi reputación
            Then veo una cobertura del 3,13 % y un promedio de 5,00

    Rule: Las páginas de reseñas no alteran los indicadores globales

        @wip
        Scenario: 72.9-RP Mantener los indicadores globales al consultar una página de reseñas
            Given que tengo tres trabajos pagados con reseñas de 2, 4 y 5 estrellas
            When consulto mi reputación con una página de una reseña
            Then la página contiene una reseña
            And los indicadores siguen considerando las 3 reseñas y muestran un promedio de 3,67

        @wip
        Scenario: 72.10-RP Ordenar por identificador y no por orden de registro de las reseñas
            Given que tengo tres trabajos pagados con reseñas, identificados como A, B y C
            And que el identificador de A es menor que el de B y el de B es menor que el de C
            And que las reseñas se registraron primero para C, luego para A y por último para B
            When consulto mis reseñas
            Then veo las reseñas de los trabajos C, B y A, en orden descendente de identificador
            And ese orden no depende del orden en que se registraron las reseñas
            And cada reseña muestra el identificador del trabajo, la calificación y la descripción registrada

        @wip
        Scenario: 72.11-RP Recorrer más de veinte reseñas sin repetir ni omitir trabajos
            Given que tengo 23 trabajos pagados, todos con reseña
            When recorro mis reseñas con el tamaño de página predeterminado
            Then la primera página contiene 20 reseñas y la siguiente contiene las 3 restantes
            And recorro los 23 trabajos una sola vez en orden descendente de identificador
            And los indicadores de ambas páginas muestran 23 reseñas y una cobertura del 100,00 %

        @wip
        Scenario: 72.12-RP Mostrar como máximo cien reseñas en una página
            Given que tengo 101 trabajos pagados, todos con reseña
            When consulto mis reseñas con un límite de 100
            Then la página contiene 100 reseñas
            And los indicadores consideran las 101 reseñas

        @wip
        Scenario Outline: 72.13-RP Rechazar una continuación inválida o incompatible
            Given que yo y "pedro@example.com" tenemos tres trabajos pagados con reseña cada uno
            And obtuve una continuación real para mis reseñas después de pedir una página de una reseña
            And "pedro@example.com" obtuvo una continuación real para sus reseñas después de pedir una página de una reseña
            And vuelvo a estar autenticado como prestador "juan@example.com"
            And <continuación>
            When intento continuar mis reseñas pidiendo una página de <tamaño de página>
            Then se me informa que la consulta no es válida y no se muestran reseñas ni indicadores

            Examples:
                | continuación                                                       | tamaño de página |
                | alteré la continuación de mi página anterior                      | una reseña       |
                | uso la continuación real emitida para "pedro@example.com"        | una reseña       |
                | uso mi continuación sin alterarla                                 | dos reseñas      |

        @wip
        Scenario: 72.14-RP Rechazar un límite superior al permitido
            When intento consultar mis reseñas con un límite de 101
            Then se me informa que la consulta no es válida y no se muestran reseñas

    Rule: Solo consulto mi reputación con una identidad válida y los errores no se confunden con datos vacíos

        @wip
        Scenario Outline: 72.15-RP Impedir la consulta sin una identidad de prestador válida
            Given que <identidad>
            When intento consultar mi reputación
            Then <resultado> y no se muestran indicadores ni reseñas

            Examples:
                | identidad                                                   | resultado                                                       |
                | no inicié sesión                                            | se rechaza la consulta porque no inicié sesión                  |
                | mi sesión no es válida                                      | se rechaza la consulta porque mi sesión no es válida            |
                | estoy autenticado como consumidor "ana@example.com"         | se rechaza la consulta porque no soy prestador                  |
                | inicié sesión con una identidad que no figura en LoResuelvo | se rechaza la consulta porque no tengo una cuenta en LoResuelvo |
