Feature: Consultar la disponibilidad de los rubros
    Como usuario de LoResuelvo
    quiero consultar los rubros habilitados para nuevas operaciones
    para elegir una oferta disponible

    Rule: El catálogo compartido sólo ofrece rubros habilitados para nuevas operaciones

        Scenario: 38.2.17-LR Listar rubros habilitados sin exponer datos administrativos
            Given que existe el rubro habilitado "Plomería"
            And que existe el rubro deshabilitado "Electricidad"
            And que estoy autenticado sin permisos administrativos
            When consulto el listado de rubros
            Then el listado incluye el rubro "Plomería" y no incluye el rubro "Electricidad"
            And cada rubro listado incluye solamente su identificador y nombre visible

        Scenario: 38.2.18-LR Consultar todos los rubros con el permiso de lectura
            Given que existe el rubro habilitado "Plomería"
            And que existe el rubro deshabilitado "Electricidad"
            And que existe un administrador provisionado con correo "supervisor@example.com", nombre "Sofía" y apellido "López"
            And que estoy autenticado como administrador "supervisor@example.com" con el permiso "read:categories"
            When consulto el listado de rubros incluyendo los deshabilitados
            Then el listado incluye ambos rubros con su estado y versión
            And la consulta no registra un evento de auditoría

        Scenario: 38.2.19-LR Rechazar el catálogo administrativo sin el permiso requerido
            Given que existe el rubro habilitado "Plomería"
            And que existe el rubro deshabilitado "Electricidad"
            And que existe un administrador provisionado con correo "supervisor@example.com", nombre "Sofía" y apellido "López"
            And que estoy autenticado como administrador "supervisor@example.com" solamente con el permiso "read:admin_operations"
            When consulto el listado de rubros incluyendo los deshabilitados
            Then el sistema responde con estado 403
