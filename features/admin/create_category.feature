Feature: Crear rubros de prestadores
    Como administrador de LoResuelvo
    quiero crear rubros en el catálogo
    para mantener actualizadas las áreas de servicio disponibles en la plataforma

    Background:
        Given que existe un administrador provisionado con correo "supervisor@example.com", nombre "Sofía" y apellido "López"

    Rule: Un administrador autorizado puede crear un rubro válido

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "create:categories"
            And que no existen rubros registrados

        Scenario: 38.1.1-CR Crear un rubro
            # US-62.1: Given que la fecha y hora actual del sistema es "2026-08-15T14:00:00Z"
            When creo el rubro "Plomería"
            Then el sistema crea el rubro
            And la respuesta contiene el identificador, el nombre "Plomería" y el nombre normalizado "plomería"
            And la ubicación del recurso creado corresponde al identificador del rubro
            # US-62.1: And queda registrado un único evento de creación exitosa para este rubro por "supervisor@example.com"
            # US-62.1: And ese evento contiene la fecha y hora "2026-08-15T14:00:00Z" en UTC y la correlación de esta solicitud

    Rule: El rubro y su auditoría se confirman conjuntamente

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "create:categories"
            And que no existen rubros registrados

        @wip
        Scenario: 62.1.1-CR No confirmar el rubro si falla el registro de auditoría
            Given que falla el almacenamiento del evento de auditoría de esta creación
            When intento crear el rubro "Plomería"
            Then el sistema responde con un error interno controlado
            And no existe el rubro "Plomería"
            And no queda registrado un evento exitoso de creación de esta solicitud

        @wip
        Scenario: 62.1.2-CR No registrar un alta si falla la persistencia del rubro
            Given que falla el almacenamiento del rubro durante esta creación
            When intento crear el rubro "Plomería"
            Then el sistema responde con un error interno controlado
            And no existe el rubro "Plomería"
            And no queda registrado un evento exitoso de creación de esta solicitud

    Rule: El nombre del rubro debe ser válido

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "create:categories"
            And que no existen rubros registrados

        Scenario: 38.1.2-CR Rechazar la creación sin nombre
            When intento crear un rubro sin nombre
            Then el sistema rechaza la creación porque el nombre del rubro es obligatorio
            # US-62.1: And no queda registrado un evento exitoso de creación de esta solicitud

        Scenario Outline: 38.1.3-CR Rechazar un nombre con una longitud inválida
            When intento crear un rubro con <nombre>
            Then el sistema rechaza la creación porque <motivo>
            # US-62.1: And no queda registrado un evento exitoso de creación de esta solicitud

            Examples:
                | nombre                      | motivo                                 |
                | el nombre vacío             | el nombre del rubro es obligatorio     |
                | solamente espacios          | el nombre del rubro es obligatorio     |
                | un nombre de 101 caracteres | el nombre del rubro es demasiado largo |

        Scenario: 38.1.4-CR Rechazar un nombre cuyo tipo no es texto
            When intento crear un rubro con un nombre numérico
            Then el sistema rechaza la creación porque el nombre del rubro debe ser texto
            # US-62.1: And no queda registrado un evento exitoso de creación de esta solicitud

    Rule: El nombre normalizado del rubro debe ser único

        Background:
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "create:categories"

        Scenario Outline: 38.1.5-CR Rechazar un rubro duplicado sin distinguir mayúsculas ni espacios exteriores
            Given que existe el rubro "Plomería"
            When intento crear el rubro <nombre duplicado>
            Then el sistema rechaza la creación porque el rubro ya existe
            # US-62.1: And no queda registrado un evento exitoso de creación de esta solicitud

            Examples:
                | nombre duplicado |
                | "Plomería"       |
                | "PLOMERÍA"       |
                | "  plomería  "   |

    Rule: Solo los usuarios con el permiso requerido pueden crear rubros

        Scenario: 38.1.6-CR Rechazar la creación sin autenticación
            Given que no tengo una sesión válida
            When intento crear el rubro "Plomería"
            Then el sistema deniega el acceso
            # US-62.1: And no queda registrado un evento exitoso de creación de esta solicitud

        Scenario: 38.1.7-CR Rechazar la creación sin el permiso requerido
            Given que estoy autenticado como administrador "supervisor@example.com" solamente con el permiso "read:providers"
            When intento crear el rubro "Plomería"
            Then el sistema responde con estado 403
            # US-62.1: And no queda registrado un evento exitoso de creación de esta solicitud
