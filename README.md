# loresuelvo-api

## Realtime entre réplicas

Los mensajes y notificaciones se distribuyen por AMQP 0.9.1 (RabbitMQ o
LavinMQ). Se reutilizan `Dispatcher`, el hub local y los payloads WebSocket.
PostgreSQL conserva los datos de negocio y los tickets de autenticación de un
solo uso, pero ya no almacena ni transporta copias de los eventos realtime.

```text
API A ── publish ──> exchange fanout externo
                         ├── cola temporal A ──> hub A ──> sockets del destinatario
                         └── cola temporal B ──> hub B ──> sockets del destinatario
```

Cada proceso mantiene una conexión AMQP con canales separados para publicar y
consumir. El broker genera una cola exclusiva y autodescartable para cada
conexión. El broadcast llega a todas las instancias; cada hub entrega únicamente
a las conexiones del usuario, rol y perfil destino. No se comparte una misma
cola entre réplicas, porque eso repartiría eventos entre consumidores.
La incorporación y salida de réplicas no requiere IPs, registro de instancias
ni cambios en Cloudflare. Una caída de conexión recrea automáticamente la cola
y su binding con backoff y jitter; la cancelación del lifecycle cierra la conexión.

La infraestructura hermana `infra-devops` ya admite varias instancias:
`terraform/environments/{staging,production}/replicas/main.tf` crea nodos con
`for_each` a partir de `replica_count`, y el flujo de escalado actualiza el pool
de Cloudflare. La afinidad de sesión no impide que dos usuarios estén en nodos
distintos. Una conexión WebSocket permanece en el nodo que aceptó su upgrade.

### Broker gratuito externo

Opciones consultadas el 5 de octubre de 2026; las cuotas pueden cambiar:

| Servicio | Plan gratuito publicado | Encaje |
| --- | --- | --- |
| [CloudAMQP RabbitMQ Little Lemur](https://www.cloudamqp.com/plans.html) | 20 conexiones, 100 colas, 10.000 mensajes en cola, 1 millón de operaciones de mensajes/mes | Recomendado para las dos instancias pedagógicas; coincide con el TP de `distri`. |
| [CloudAMQP LavinMQ Loyal Lemming](https://www.cloudamqp.com/plans.html) | 40 conexiones, 200 colas, 20.000 mensajes en cola, 2 millones de operaciones/mes | Alternativa AMQP compatible con este adaptador. |
| [Upstash Redis Free](https://upstash.com/pricing/redis) | 500.000 comandos/mes, 256 MB, 10 GB/mes | Pub/Sub es viable, pero requiere otro cliente y adaptador. |

CloudAMQP [cuenta publicación y consumo](https://www.cloudamqp.com/docs/faq.html)
por separado. Con dos instancias, un envelope consume aproximadamente tres
operaciones: una publicación y dos recepciones. Little Lemur permite así unas
333.000 publicaciones mensuales sin otros consumos ni reentregas. Un acto de
negocio puede generar varios envelopes. Se usan dos conexiones de las veinte;
hay margen para reconexiones y herramientas. Los planes compartidos se ofrecen
para desarrollo/proyectos pequeños; verificar el plan y sus límites al crearlo.

### Configuración y despliegue

1. Crear un broker **Little Lemur gratuito** en CloudAMQP, fuera de las VMs de API.
2. Guardar su URL `amqps://usuario:password@host/vhost` como `AMQP_URL` en los
   secretos de API que exporta `infra-devops`. No guardar credenciales en Git.
3. Usar el mismo `AMQP_URL` y `AMQP_EXCHANGE` en ambas instancias. El exchange
   predeterminado es `loresuelvo.realtime`; separar staging/producción con
   brokers/vhosts distintos o, como mínimo, exchanges distintos.
4. Desplegar la misma versión en ambas instancias. Ansible ya instala secretos
   en `/etc/loresuelvo/api/api.env` y Compose consume ese archivo; no hace falta
   agregar un servidor ni abrir puertos entrantes en las VMs. Se necesita salida
   al broker por TLS, habitualmente TCP 5671.
5. Verificar el log `AMQP realtime subscribed` en ambos nodos y enviar un mensaje
   con emisor y destinatario conectados a nodos diferentes.

La API exige una URL válida y TLS para conexiones remotas. Solo se permite
`amqp://` para loopback y el servicio local `rabbitmq`. Durante un despliegue
mixto, las versiones antiguas que usan PostgreSQL y las nuevas que usan AMQP
no intercambian eventos: coordinar el reemplazo de los dos nodos. La migración
histórica de `realtime_events` permanece intacta para permitir rollback; esta
versión no lee ni escribe esa tabla.

### Alcance de entrega

El publish espera confirmación del broker con un límite de cinco segundos.
Eso no confirma lectura ni entrega al cliente. El dispatcher deduplica el eco
del broker y las reentregas en memoria durante diez minutos, hasta 10.000 IDs.
Las colas limitan el backlog a 1.000 eventos y caducan eventos a los 60 segundos.

El aviso realtime es transitorio: no hay replay ni garantía exactamente una vez.
Una caída del broker, desconexión de réplica o desborde puede perder avisos,
aunque los mensajes de negocio ya confirmados siguen en la base. Los clientes
deben refrescar historial/notificaciones al reconectar y reconciliar si necesitan
recuperar avisos perdidos mientras el WebSocket siguió abierto. No hay fallback
a PostgreSQL ni reintento automático de publicaciones de resultado incierto.
La readiness HTTP actual verifica la base, no la suscripción AMQP; revisar el
log de suscripción antes de probar realtime tras un arranque.

### Desarrollo y pruebas

El RabbitMQ de `docker-compose.yml` es exclusivamente local. `make up` lo inicia;
no se despliega en las instancias de producción. Para preparar las pruebas sin
arrancar toda la API:

```bash
docker compose up -d --wait rabbitmq test-db minio
docker compose run --rm --no-deps minio-init
make test-ci
docker compose run --rm --no-deps -e GOFLAGS=-buildvcs=false api-dev \
  go test -race -count=1 ./internal/adapters/realtime -run 'TestAMQP|TestDispatcher'
```

`TEST_AMQP_URL` activa las pruebas contra el broker real y está configurada en
Compose/CI; si está presente, un broker inaccesible hace fallar las pruebas.
Los escenarios BDD de una sola instancia usan el camino local del dispatcher.
La integración AMQP comprueba broadcast entre dos hubs, aislamiento del usuario,
deduplicación, payloads mayores de 8 KB, reconexión y cancelación del handshake.

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

## Evaluaciones del chatbot

El [golden dataset y las instrucciones de evaluación US-60](evals/README.md)
se mantienen separados de las pruebas funcionales de la API.
