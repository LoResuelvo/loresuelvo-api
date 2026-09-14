# Aprovisionamiento del administrador

Procedimiento operativo para la US 3.2. El acceso combina una identidad
gestionada por Auth0 con un perfil local en `users`; ninguno de los dos
sustituye al otro.

## Alcance y límites

- Auth0 administra la identidad, la contraseña, el MFA y el rol/permisos del
  usuario. La API sólo aprovisiona el perfil local asociado al `user_id`.
- El aprovisionamiento local es una semilla **obligatoria, automática e
  idempotente** del arranque de la API. No requiere Management API, scripts ni
  una integración adicional con Auth0.
- No se guardan contraseñas de Auth0 en PostgreSQL.
- La existencia del perfil local permite resolver `/me`, pero **no concede por
  sí sola autorización administrativa**. Los endpoints de las US 61 y 38.1
  deben exigir sus permisos correspondientes en la API.
- La autenticación real, el MFA y la configuración del tenant se verifican
  manualmente en Auth0; no se simulan en los escenarios Godog.

## 1. Crear y proteger la identidad en Auth0

En el tenant de cada entorno:

1. Usar una aplicación separada para la web Admin como **Regular Web
   Application (RWA)**, con Authorization Code. Mantener el `client_secret`
   exclusivamente en el servidor y no exponerlo en el navegador. Configurar en
   la aplicación las URLs exactas de callback, logout y origen web de cada
   entorno; no usar comodines en producción.
2. Habilitar únicamente la conexión destinada a administradores, mantener el
   registro público deshabilitado y usar usuarios individuales (sin
   Organizations) para este caso.
3. Crear explícitamente la cuenta inicial desde el Dashboard de Auth0. Usar
   el correo corporativo acordado, verificarlo y exigir MFA según la política
   del tenant.
4. En el API de Auth0, habilitar **Enable RBAC** y **Add Permissions in the
   Access Token**. Crear el rol `admin`, asociarle sólo los permisos
   administrativos definidos por la API y asignar ese rol a la cuenta desde la
   pestaña **Roles** del usuario. La API debe autorizar por los permisos del
   access token, nunca por el `role` local, un claim `role` ni un dato del ID
   token.
5. Copiar el **User ID** completo de Auth0 (por ejemplo,
   `auth0|...`). Ese valor será `users.auth_id` y debe conservarse exactamente,
   incluido el prefijo del proveedor.

No convertir automáticamente en administrador al primer usuario que inicie
sesión ni reutilizar una cuenta existente sólo porque coincide el correo.

El servidor de la RWA usa el **access token** con la audiencia de nuestra API en
las llamadas al backend; el navegador no recibe ni envía ese token directamente.
El **ID token** sólo representa la sesión y contiene información de identidad
para la aplicación cliente; no debe enviarse a la API como token de autorización.
Referencias: [Authorization Code para Regular Web Apps](https://auth0.com/docs/get-started/authentication-and-authorization-flow/authorization-code-flow/add-login-auth-code-flow),
[configuración de URLs de la aplicación](https://auth0.com/docs/get-started/applications/application-settings),
[RBAC y permisos en el access token](https://auth0.com/docs/get-started/apis/enable-role-based-access-control-for-apis),
y [asignación de roles a usuarios](https://auth0.com/docs/manage-users/access-control/configure-core-rbac/rbac-users/assign-roles-to-users).

## 2. Repetir el procedimiento por entorno

Staging y producción deben usar su propia combinación de tenant, dominio,
audience/API, Client ID, URLs de callback y base de datos. El `user_id` de
Auth0 es emitido por un tenant y **no es portable** a otro: no copiar el
usuario ni su `auth_id` de producción a staging (ni al revés). Crear o
verificar la cuenta en el tenant correspondiente y copiar el User ID desde su
propia vista de usuario antes de ejecutar la vinculación local.

Desplegar la API y la RWA con la configuración del entorno antes de hacer una
prueba real. La API debe tener `AUTH0_DOMAIN` y `AUTH0_AUDIENCE` del mismo
tenant/API que emitió el access token.

Las pruebas automatizadas usan el validador falso del proyecto y fixtures; no
prueban el tenant, MFA ni una RWA real. Después del despliegue, la comprobación
manual mínima de `GET /me` es:

| Caso | Resultado esperado |
| --- | --- |
| Access token válido del admin y fila local vinculada | `200` |
| Sin Bearer o con token inválido | `401` |
| Access token válido de una identidad sin fila local | `404` |

Para el tercer caso usar una identidad temporal sin aprovisionamiento local;
no borrar ni modificar el administrador real para fabricar la prueba.

## 3. Configurar la semilla obligatoria por entorno

Antes de iniciar la API, configurar estas variables en el mecanismo de
secretos/configuración del entorno:

| Variable | Tipo | Uso |
| --- | --- | --- |
| `ADMIN_SEED_AUTH_ID` | **secreto** | `user_id` completo emitido por Auth0, incluido el prefijo (`auth0\|...`) |
| `ADMIN_SEED_EMAIL` | **secreto** | Correo de la identidad creada en Auth0 |
| `ADMIN_SEED_NAME` | configuración | Nombre del perfil local |
| `ADMIN_SEED_SURNAME` | configuración | Apellido del perfil local |

En local, se pueden definir en el archivo `.env` ignorado por Git. Los valores
reales de `ADMIN_SEED_AUTH_ID` y `ADMIN_SEED_EMAIL` no deben aparecer en el
repositorio, logs, imágenes ni documentación. Staging y producción deben
obtenerlos desde su gestor de secretos; la configuración no es portable entre
entornos.

La API lee las cuatro variables y ejecuta la semilla dentro de una transacción
antes de abrir el servidor HTTP. La semilla:

- crea el perfil con `role = 'admin'` cuando no existe;
- no depende de `SEEDS_ENABLED` ni de ningún archivo YAML de proveedores;
- repetir el arranque con los mismos valores es un no-op;
- falla el arranque si falta una variable, un valor no es válido, el
  `auth_id` ya pertenece a otro correo/rol, o el correo ya pertenece a otra
  identidad;
- nunca promueve, reasigna ni sobrescribe un usuario existente.

Por lo tanto, no se debe ejecutar una query SQL manual para aprovisionar el
admin. Si la semilla falla, corregir la configuración o el conflicto y volver
a iniciar la aplicación; no resolverlo mediante un `UPDATE` directo.

## 4. Verificación funcional

Iniciar sesión en la RWA Admin con la cuenta creada y MFA. El servidor debe
obtener un access token para la API y consultar `GET /me` con
`Authorization: Bearer <token>`. La respuesta debe corresponder al perfil local
creado. Un token válido sin fila local debe responder como usuario no
encontrado; no se debe crear el perfil implícitamente.

Después, validar por separado los permisos de cada endpoint administrativo
cuando se implementen las US 61 y 38.1. Un `role` devuelto por `/me` es dato de
perfil, no una prueba suficiente de autorización.
