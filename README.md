# loresuelvo-api

## Gateway de desarrollo y túneles

El servicio `nginx-dev` ofrece un único punto de entrada para el frontend y la
API. Envía `/api/*` (quitando el prefijo `/api`), las rutas actuales de la API y
los endpoints públicos `/oauth`, `/webhooks` y `/ws` a `api-dev`; cualquier otra
ruta se envía al frontend que corre en el host, por defecto en el puerto `3000`.
El frontend puede usar `/api` como URL base. Los controles `/api/test/*` no se
publican a través del gateway.

1. Levantar el frontend en el host.
2. Definir la URL HTTPS asignada por el túnel y levantar el gateway:

   ```bash
   DEV_PUBLIC_URL=https://example.ngrok-free.app make dev-proxy
   ```

3. Apuntar el túnel a `http://localhost:8082`.

`DEV_PUBLIC_URL` configura en conjunto el callback OAuth, el webhook y los
retornos de Checkout Pro de Mercado Pago. Debe definirse al crear o recrear
`api-dev`. Si el túnel cambia, volver a ejecutar el comando anterior. La misma
URL, con `/oauth/payment-accounts/callback`, debe estar registrada como redirect
URI en la aplicación de Mercado Pago.

Variables opcionales:

- `DEV_WEB_PORT`: puerto del frontend en el host (por defecto `3000`).
- `NGINX_DEV_PORT`: puerto local del gateway (por defecto `8082`).
- Las variables específicas `MERCADO_PAGO_REDIRECT_URI`,
  `MERCADO_PAGO_NOTIFICATION_URL` y `PAYMENT_*_URL` tienen prioridad sobre los
  valores derivados de `DEV_PUBLIC_URL`.
