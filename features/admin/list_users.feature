@wip
Feature: Listar usuarios registrados
    Como administrador de LoResuelvo
    quiero consultar por separado a consumidores y prestadores
    para supervisar la operación del marketplace

    Background:
        Given que existe un administrador provisionado con correo "supervisor@example.com", nombre "Sofía" y apellido "López"

    Rule: El directorio de consumidores debe incluir solamente consumidores

        Scenario: 61.1-LU Listar todos los consumidores registrados
            Given que existe el rubro "Plomería"
            And que existen los siguientes consumidores registrados:
                | correo              | nombre  | apellido |
                | ana@example.com     | Ana     | Pérez    |
                | beatriz@example.com | Beatriz | Suárez   |
            And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:consumers"
            When consulto el directorio de consumidores
            Then el directorio de consumidores contiene exactamente a:
                | correo              |
                | ana@example.com     |
                | beatriz@example.com |
            And el directorio de consumidores no incluye a "supervisor@example.com"
            And el directorio de consumidores no incluye a "juan@example.com"

        Scenario: 61.2-LU Informar los datos administrativos de cada consumidor
            Given que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And que "ana@example.com" tiene una foto de perfil pública
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:consumers"
            When consulto el directorio de consumidores
            Then el consumidor "ana@example.com" incluye su identificador, rol, nombre, apellido, correo y fecha de registro
            And el consumidor "ana@example.com" incluye la URL pública de su foto de perfil
            And el consumidor "ana@example.com" no expone credenciales ni identificadores del proveedor de identidad

        Scenario: 61.3-LU Devolver un directorio de consumidores vacío
            Given que no existen consumidores registrados
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:consumers"
            When consulto el directorio de consumidores
            Then el sistema devuelve un directorio de consumidores vacío

    Rule: El directorio de prestadores debe incluir su información operativa

        Scenario: 61.4-LU Listar todos los prestadores registrados
            Given que existe el rubro "Plomería"
            And que existen los siguientes prestadores registrados en el rubro "Plomería":
                | correo            | nombre | apellido |
                | juan@example.com  | Juan   | Gómez    |
                | laura@example.com | Laura  | Díaz     |
            And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:providers"
            When consulto el directorio de prestadores
            Then el directorio de prestadores contiene exactamente a:
                | correo            |
                | juan@example.com  |
                | laura@example.com |
            And el directorio de prestadores no incluye a "ana@example.com"
            And el directorio de prestadores no incluye a "supervisor@example.com"

        Scenario: 61.5-LU Informar el rubro y todas las zonas de cobertura del prestador
            Given que existe el rubro "Plomería"
            And que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14"
            And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que "juan@example.com" cubre las zonas "Comuna 6" y "Comuna 14"
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:providers"
            When consulto el directorio de prestadores
            Then el prestador "juan@example.com" incluye el rubro "Plomería"
            And el prestador "juan@example.com" incluye exactamente las siguientes zonas de cobertura:
                | zona      |
                | Comuna 6  |
                | Comuna 14 |
            And cada zona incluida informa su identificador, market, código, nombre, tipo, zona padre y disponibilidad

        Scenario: 61.6-LU Informar la foto y el último estado de verificación del prestador
            Given que existe el rubro "Plomería"
            And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que "juan@example.com" tiene una foto de perfil pública
            And que la verificación de "juan@example.com" estuvo "in_review" el "2026-09-18T14:00:00Z"
            And que la identidad de "juan@example.com" fue aprobada el "2026-09-18T15:30:00Z"
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:providers"
            When consulto el directorio de prestadores
            Then el prestador "juan@example.com" incluye su identificador, rol, nombre, apellido, correo y fecha de registro
            And el prestador "juan@example.com" incluye la URL pública de su foto de perfil
            And el prestador "juan@example.com" informa el estado de verificación "approved"
            And el prestador "juan@example.com" informa la fecha de verificación "2026-09-18T15:30:00Z"
            And el prestador "juan@example.com" no expone credenciales, identificadores del proveedor de identidad, documentos ni códigos de riesgo

        Scenario: 61.7-LU Informar como no verificado a un prestador sin sesiones de verificación
            Given que existe el rubro "Plomería"
            And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And que "juan@example.com" no tiene sesiones de verificación de identidad
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:providers"
            When consulto el directorio de prestadores
            Then el prestador "juan@example.com" informa el estado de verificación "unverified"
            And el prestador "juan@example.com" no informa una fecha de verificación

        Scenario: 61.8-LU Devolver un directorio de prestadores vacío
            Given que no existen prestadores registrados
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:providers"
            When consulto el directorio de prestadores
            Then el sistema devuelve un directorio de prestadores vacío

    Rule: El administrador puede buscar usuarios por nombre, apellido o correo sin distinguir mayúsculas

        Background:
            Given que existe el rubro "Plomería"
            And que existe un consumidor registrado con correo "ana.perez@example.com", nombre "Ana" y apellido "Pérez"
            And que existe un consumidor registrado con correo "beatriz@example.com", nombre "Beatriz" y apellido "Suárez"
            And existe un prestador registrado con correo "juan.gomez@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And existe un prestador registrado con correo "laura@example.com", nombre "Laura", apellido "Díaz" y rubro "Plomería"

        Scenario Outline: 61.9-LU Buscar <tipo de usuario> por <campo>
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "<permiso>"
            When busco <tipo de usuario> con el texto "<búsqueda>"
            Then el directorio contiene solamente a "<correo esperado>"

            Examples:
                | tipo de usuario | permiso        | campo    | búsqueda              | correo esperado        |
                | consumidores    | read:consumers | nombre   | ANA                   | ana.perez@example.com  |
                | consumidores    | read:consumers | apellido | PÉREZ                 | ana.perez@example.com  |
                | consumidores    | read:consumers | correo   | ANA.PEREZ@EXAMPLE.COM  | ana.perez@example.com  |
                | prestadores     | read:providers | nombre   | JUAN                  | juan.gomez@example.com |
                | prestadores     | read:providers | apellido | GÓMEZ                 | juan.gomez@example.com |
                | prestadores     | read:providers | correo   | JUAN.GOMEZ@EXAMPLE.COM | juan.gomez@example.com |

        Scenario Outline: 61.10-LU Devolver un directorio vacío cuando la búsqueda no tiene coincidencias
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "<permiso>"
            When busco <tipo de usuario> con el texto "inexistente"
            Then el sistema devuelve un directorio de <tipo de usuario> vacío

            Examples:
                | tipo de usuario | permiso        |
                | consumidores    | read:consumers |
                | prestadores     | read:providers |

    Rule: El administrador puede filtrar prestadores por sus atributos operativos

        Background:
            Given que existen los rubros "Plomería" y "Electricidad"
            And que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14"
            And existe un prestador registrado con correo "juan@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
            And existe un prestador registrado con correo "laura@example.com", nombre "Laura", apellido "Díaz" y rubro "Electricidad"
            And que "juan@example.com" cubre la zona "Comuna 6"
            And que "laura@example.com" cubre la zona "Comuna 14"
            And que la identidad de "juan@example.com" está aprobada
            And que la verificación de "laura@example.com" está en estado "declined"
            And existe un prestador registrado con correo "pedro@example.com", nombre "Pedro", apellido "Ruiz" y rubro "Plomería"
            And que "pedro@example.com" no tiene sesiones de verificación de identidad
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:providers"

        Scenario: 61.11-LU Filtrar prestadores por rubro
            When filtro el directorio de prestadores por el rubro "Electricidad"
            Then el directorio contiene solamente a "laura@example.com"

        Scenario: 61.12-LU Filtrar prestadores por zona de cobertura
            When filtro el directorio de prestadores por la zona de cobertura "Comuna 6"
            Then el directorio contiene solamente a "juan@example.com"

        Scenario Outline: 61.13-LU Filtrar prestadores por estado de verificación <estado>
            When filtro el directorio de prestadores por el estado de verificación "<estado>"
            Then el directorio contiene solamente a "<correo esperado>"

            Examples:
                | estado     | correo esperado   |
                | approved   | juan@example.com  |
                | declined   | laura@example.com |
                | unverified | pedro@example.com |

    Rule: Los filtros inválidos deben rechazarse

        Scenario Outline: 61.14-LU Rechazar un filtro inválido de prestadores
            Given que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:providers"
            When consulto el directorio de prestadores con el parámetro "<parámetro>" igual a "<valor>"
            Then el sistema responde con estado 400
            And el sistema informa que el filtro es inválido

            Examples:
                | parámetro                    | valor         |
                | category_id                  | no-numérico   |
                | coverage_zone_id             | 0             |
                | identity_verification_status | desconocido   |

        Scenario Outline: 61.15-LU Rechazar la consulta sin el permiso requerido
            Given que estoy autenticado como administrador "supervisor@example.com" solamente con el permiso "<permiso disponible>"
            When intento consultar el directorio de <tipo de usuario>
            Then el sistema responde con estado 403

            Examples:
                | tipo de usuario | permiso disponible |
                | consumidores    | read:providers      |
                | prestadores     | read:consumers      |
