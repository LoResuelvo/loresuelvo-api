# Evaluaciones US-60

## Golden dataset

[`datasets/LoResuelvo_US60_evals_v1.0.0/`](datasets/LoResuelvo_US60_evals_v1.0.0/README.md)
contiene entradas, expectativas, imágenes y políticas para tres tareas independientes:
prediagnóstico, ranking y resumen.

- 70 casos: 47 PD, 19 RK y 4 SM.
- Los 70 casos están aprobados.
- 51 casos de desarrollo y 19 de reserva.
- Ocho imágenes sintéticas y ninguna campaña.
- Smoke: 17 casos de desarrollo; `critical_all`: 13 PD críticos.

El manifiesto registra el inventario exacto con hashes SHA-256. Los esquemas
permiten comprobar la estructura; no reemplazan evaluación semántica.
Los casos de resumen verifican acciones y preferencias, actualización temporal,
incertidumbre causal y asociación de referencias visuales.

La reserva no se usa para ajustar la solución. Variantes y repeticiones no son
observaciones independientes. No se envían al modelo expectativas, criterios,
splits ni procedencia. La aprobación del corpus no aprueba futuras respuestas
ni autoriza un release del chatbot.

## Solución evaluada

`cmd/evals` usa el mismo adaptador Gemini que la API para
`AnswerHomeProblemQuestion`, `RankProviders` y `SummarizeHomeProblemConversation`.
La base utiliza los prompts iniciales con el esquema de respuesta y el parser
actuales. El prompt no identifica por sí solo la solución: también cuentan el
modelo, el esquema enviado, la configuración, el mapeo y la validación de salida.
Los registros vinculan código, build, dataset, entrada efectiva y configuración
mediante commits y hashes.

La evaluación es a nivel adaptador, con categorías y candidatos controlados;
no reproduce búsquedas en la base de datos ni el flujo HTTP completo. SM genera
un resumen a partir del resumen previo y los mensajes del fixture. Los criterios
esperados no se entregan al modelo.

El protocolo fija 4096 tokens de salida por generación, incluido el pensamiento;
puede truncar una respuesta con `MAX_TOKENS`. Se comprueba en smoke antes de una
campaña completa, sin garantizar que alcance para todos los casos. Ver
[pensamiento de Gemini](https://ai.google.dev/gemini-api/docs/generate-content/thinking).
La API no fija ese límite explícitamente; esta diferencia se registra, no se
presenta como equivalencia de configuración. Los defaults del proveedor que no
se observan permanecen desconocidos.

## Primera campaña

[`protocols/campaign-1.json`](protocols/campaign-1.json) fija la primera medición
de desarrollo. Seguir el [procedimiento de preparación, smoke y campaña](protocols/README.md):
primero previsualización offline, luego smoke independiente autorizado e inspección,
y sólo después autorización separada de la campaña. Versionar el protocolo no
inicia llamadas ni aprueba respuestas.

## Ejecución manual

Desde la raíz del repositorio, sin credenciales ni llamadas al proveedor:

```bash
go run ./cmd/evals validate
go run ./cmd/evals plan --suite smoke --model gemini-2.5-flash --max-requests 17
go run ./cmd/evals replay --run RUTA_CORRIDA
go run ./cmd/evals compare --left RUTA_BASE --right RUTA_CANDIDATA
```

`plan` y `--dry-run` previsualizan la ejecución. `replay` reevalúa respuestas
guardadas: no vuelve a probar el modelo ni el prompt. `compare` exige corridas
compatibles y una declaración explícita de cambios deliberados. La ayuda de
`go run ./cmd/evals --help` detalla comandos y flags.

Las llamadas reales requieren `--allow-live`, límites finitos de solicitudes y
tiempo, y credenciales. `campaign-live` además requiere protocolo y verificación
fechada de precios; su conteo previo de tokens también contacta al proveedor.
La reserva requiere autorización adicional (`--allow-holdout`). Las evals no se
ejecutan automáticamente en CI ni al correr las pruebas ordinarias.

Para live, usar código versionado y checkout limpio: construir el paquete con
`go build -o tmp/evals-runner ./cmd/evals` o usar `go run ./cmd/evals`, preservando
metadata VCS de la misma revisión. Escribir `--out` en una ruta ignorada como
`tmp/evals-...` o fuera del repositorio, no dentro del dataset ni en una ruta que
ensucie el checkout entre fases.

### Claves y concurrencia

El ejecutor actual es Gemini. `CHATBOT_EVALS_API_KEYS` admite una lista separada
por comas, independiente de `CHATBOT_API_KEY`, que pertenece a la API. Configurar
valores reales sólo en el entorno o en archivos ignorados; nunca versionarlos.

La concurrencia no supera la cantidad de claves configuradas. Para aumentar
capacidad, cada clave debe pertenecer a un proyecto distinto: Gemini aplica
cuotas por proyecto, no por clave. Ver [límites oficiales](https://ai.google.dev/gemini-api/docs/rate-limits).
El pacing por proyecto se conserva entre fases; una guardia global reserva
presupuesto antes de cada solicitud y un único escritor mantiene el journal.
Un reintento o recuperación corrige un fallo técnico, no selecciona la respuesta
con mejor puntuación.

## Métricas y evidencia

- **Operación:** todos los intentos físicos de generación, incluidos fallos, reintentos y
  recuperaciones; totales y desgloses de ejecución original y recuperación.
- **Calidad:** un resultado efectivo por caso/trial/variante, sin borrar los
  intentos descartados. Casos base, repeticiones y variantes tienen denominadores
  distintos; no se suman como casos independientes.
- **Uso:** tokens de entrada, salida, pensamiento, caché y total con fuente y
  cobertura. Un cero informado es `0`; un dato ausente es `null` con motivo.
  En Gemini, la entrada incluye los tokens cacheados y el total incluye entrada,
  salida y pensamiento; no volver a sumarlos. Ver [UsageMetadata](https://ai.google.dev/api/generate-content#UsageMetadata).
- **Tiempo y diagnóstico:** latencia del intento (incluye mapeo, generación y
  parseo), ventana temporal de las corridas de generación, modelo
  solicitado/resuelto, IDs HTTP y de respuesta, estado HTTP y motivos de
  terminación/bloqueo cuando se informan. No se mide tiempo al primer token.
- **Costo:** estimación por modelo y uso observado con snapshot de tarifa,
  fuente, fecha, modalidad de cobro, moneda y cobertura. Una tarifa o un uso
  insuficientes producen `null`, no cero. La reserva presupuestaria no es costo
  observado; el importe facturado es desconocido sin comprobación externa.
  Las modalidades de tarifa se consultan en [precios oficiales](https://ai.google.dev/gemini-api/docs/pricing).

`CountTokens` tiene un contador separado y participa en los controles de
presupuesto y pacing. No dispone de ledger individual con ID/latencia y no
entra en las distribuciones de latencia ni en el costo de generación. Su
preflight anterior a la primera corrida queda fuera de la ventana temporal
reportada; ésta no mide todo el pipeline. No se inventa un costo auxiliar cero.

Para cuota gratuita, declarar `--billing-tier free` en `live` y
`execution.billing_tier: "free"` en el protocolo de campaña. Una tarifa paga
para comparar consumo usa `basis: "standard_paid_reference"`: representa
**costo de referencia**, no un cargo real. Ni esa estimación ni la reserva
presupuestaria prueban facturación; el importe facturado permanece desconocido.

Los criterios semánticos se registran como `pass`, `fail`, `unassessed` o
`not_applicable`, vinculados al criterio y a la salida evaluada. JSON válido no
acredita seguridad ni fidelidad. Los fallos y las respuestas ausentes permanecen
visibles; no se convierten en aprobaciones. Los umbrales empíricos no están
calibrados y cero fallos observados no demuestra riesgo cero. Un criterio crítico
fallido o sin evaluar impide declarar aprobado un release; el smoke no acredita
los críticos de reserva que no ejecutó.

El criterio compartido `shared_readability` (`minor`) evalúa semánticamente la
legibilidad en PD, RK y SM: saltos entre secciones, cada paso o elemento de lista
en su propia línea y párrafos breves cuando el contenido contiene varias ideas.
Se juzga cada campo de prosa por separado y proporcionalmente: un párrafo breve
es válido. No considera espacios del JSON ni identificadores. Sin prosa evaluable,
el revisor puede marcar `not_applicable`; Go y Python lo dejan `unassessed` hasta
ese juicio, también ante salidas ausentes. No sustituye los criterios de seguridad.
Ver [alcance y política](datasets/LoResuelvo_US60_evals_v1.0.0/README.md#legibilidad-compartida--2026-10-19).
Su integración reevalúa trials guardados sin modificar el prompt, la solución,
las entradas o expectativas de los casos ni generar nuevas respuestas. Los
juicios anteriores se conservan para trazabilidad; no publica la primera campaña.

## Artefactos de campaña

Cada campaña conserva su protocolo y configuración, además del export:

| Archivo | Contenido |
|---|---|
| `manifest.json` | Identidad, fuentes, commits/builds, planes y configuraciones por hash; inventario del export. |
| `attempts.jsonl` | Todos los intentos físicos de generación, estados, tiempos, uso y vínculos de reintento/recuperación. |
| `responses.jsonl` | Todas las respuestas de generación recibidas, texto crudo intacto y proyección válida cuando corresponde. |
| `results.jsonl` | Resultado efectivo por slot, comprobaciones y resultados semánticos por ID/hash. |
| `summary.json` | Agregados de calidad, cobertura, operación, presupuesto y estimación de costo. |
| `SHA256SUMS` | Integridad de los artefactos. |
| `README.md` | Instrucciones de lectura y reproducción de esa campaña. |

Las respuestas faltantes tienen motivo explícito. El export público no contiene
identidades ni razonamientos privados de revisión, credenciales, cabeceras
completas ni volcados HTTP. Configuraciones y referencias se deduplican por hash;
las estadísticas y gráficas se derivan de los registros, no los sustituyen.
Los resultados se escriben fuera del paquete del dataset y no modifican sus
expectativas para hacer aprobar una salida.
