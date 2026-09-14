# Evaluaciones US-60

**Estado: golden dataset preparado; el bundle canónico de Campaign 1 está pendiente
de exportación y validación.** No hay umbrales empíricos calibrados: la validación
de los materiales no demuestra calidad ni seguridad del chatbot.

## Dataset canónico

[`datasets/LoResuelvo_US60_evals_v1.0.0/`](datasets/LoResuelvo_US60_evals_v1.0.0/README.md)
es la única fuente de casos, imágenes, expectativas, políticas y particiones.
Su `manifest.json` identifica el contenido mediante SHA-256; las campañas deben
registrar ese hash, no identificar el corpus sólo por su nombre o versión.

- 48 casos de prediagnóstico y 24 de ranking: 54 de desarrollo y 18 de reserva.
- 12 especificaciones de contrato, separadas de las observaciones de calidad del LLM.
- Ocho imágenes: seis escenas sintéticas fotorrealistas, un control blanco y uno de texto adversarial.
- Smoke: 18 casos de desarrollo. `critical_all`: 13 casos de prediagnóstico de riesgo crítico, incluidos casos de reserva.

El [README del dataset](datasets/LoResuelvo_US60_evals_v1.0.0/README.md) documenta
procedencia, criterios y métricas. Durante una comparación no se modifican las
entradas, los adjuntos ni las expectativas. El ajuste utiliza desarrollo, no
reserva; los casos de reserva utilizados para ajustar una solución no constituyen
una prueba independiente de generalización.

## Validación offline

Desde la raíz del repositorio:

```bash
python3 -m venv evals/.venv
evals/.venv/bin/python -m pip install -r evals/datasets/LoResuelvo_US60_evals_v1.0.0/requirements.txt
PYTHONDONTWRITEBYTECODE=1 evals/.venv/bin/python evals/datasets/LoResuelvo_US60_evals_v1.0.0/tools/evalpack.py validate
PYTHONDONTWRITEBYTECODE=1 evals/.venv/bin/python -m unittest discover -s evals/datasets/LoResuelvo_US60_evals_v1.0.0/tests -v
make evals-validate
```

La instalación puede necesitar red; los validadores no cargan credenciales ni
llaman al modelo. Python verifica integridad y consistencia editorial; Go verifica
integridad, esquemas locales, referencias y mapeo a contratos de dominio.
Las pruebas unitarias usan salidas sintéticas controladas, no resultados de campañas.

## Implementación y límites

- `cmd/evals/`: CLI, selección explícita de comandos y autorización de llamadas.
- `internal/evals/`: carga, planificación, contratos, puntuación, diarios, reproducción y comparación.
- `internal/adapters/chatbot/`: adaptadores de producción, configuración de generación y observación.
- `evals/runs/<run_id>/`: salidas locales generadas, ignoradas por Git.

PD/RK utilizan los adaptadores y prompts de producción, sin un prompt alternativo
para aprobar el benchmark. Los metadatos del evaluador y las expectativas nunca
se envían al modelo. Las imágenes nuevas se cargan como bytes verificados; una
imagen ausente no se sustituye por su descripción esperada. Las imágenes históricas
conservan su ID y descripción y no se reenvían como adjuntos nuevos.

```bash
make evals-contract
```

Los contratos son offline. CT-02 verifica el límite servicio/repositorio, no el
filtrado SQL real. CT-08 y CT-10 distinguen evidencia estructural de semántica:
las respuestas controladas no certifican detección de riesgo ni fidelidad de un
resumen generado. Los aspectos no demostrados permanecen `unassessed`.

## Planificación

Defina `MODEL` con el identificador explícito del modelo que se vaya a evaluar.
El modelo no se toma implícitamente de la configuración del despliegue.

### Protocolo de campaña 1

[`protocols/campaign-1.json`](protocols/campaign-1.json) es la especificación
declarativa de la primera campaña. Evalúa únicamente el split `development` del
golden dataset y la solución original identificada por `ca5e2ae`, sin cambios de
prompt, configuración de generación ni correcciones orientadas a calidad.

Incluye dos modelos (`gemini-3.1-flash-lite` y `gemini-3.5-flash-lite`), tres
ensayos por cada uno de los 54 casos de desarrollo (324 llamadas en total) y un
smoke de 18 casos por modelo (36 llamadas adicionales). La batería metamórfica
queda fuera de esta campaña. Los contratos y los cinco baselines de ranking se
ejecutan offline una sola vez y no consumen llamadas al proveedor.

El techo duro de la campaña es USD 10, incluyendo smoke y cualquier reintento
autorizado. Antes de la primera llamada se debe verificar un límite superior
conservador usando el precio vigente y el máximo de tokens de entrada/salida; si
no puede demostrarse que el plan cabe en el techo, no se llama al proveedor.
Tokens, precios o costes no informados se registran como desconocidos, no como
cero. Las respuestas inválidas, errores operativos y posiciones no ejecutadas se
conservan en el denominador y no se repiten para mejorar calidad.

Los comandos de planificación genérica no ejecutan esta campaña. Los ensayos
predeterminados proceden de
`configs/experiment.json` dentro del dataset: uno para smoke y tres para desarrollo
y reserva. `--trials` permite una anulación explícita. El presupuesto incluye
reintentos; `--max-retries` vale cero por defecto. Un plan que excede el presupuesto
se rechaza, no se trunca silenciosamente.

`--allow-holdout` es obligatorio para reserva o `critical_all` y no sustituye la
autorización live. `--cases` sólo restringe los miembros de la suite elegida.

## Ejecución autorizada y trazas

Antes de llamar al proveedor se requiere un árbol limpio y versionado, un protocolo
acordado y autorización explícita. Configure `CHATBOT_API_KEY` mediante el mecanismo
local de credenciales; la CLI no carga `.env`. No incluya secretos en argumentos,
commits o informes. La existencia de la clave no autoriza llamadas.

La única ruta autorizada para ejecutar la campaña es `campaign-live`, con el
protocolo versionado, verificación de precios y una salida privada nueva:

```bash
go run ./cmd/evals campaign-live --protocol evals/protocols/campaign-1.json --allow-live --pricing-verified-on YYYY-MM-DD --out PRIVATE_NEW_DIR
```

Después de completar las revisiones, generar el informe exclusivamente con
`campaign-report`, manteniendo la evidencia y el informe fuera de Git:

```bash
go run ./cmd/evals campaign-report --protocol evals/protocols/campaign-1.json --evidence PRIVATE_NEW_DIR/evidence.json --out PRIVATE_REPORT_DIR
```

### Recuperación acotada

[`protocols/campaign-1-recovery-addendum.json`](protocols/campaign-1-recovery-addendum.json)
autoriza únicamente completar trials sin respuesta. Nunca se reintentan respuestas
recibidas aunque sean inválidas o de baja calidad. Se admiten errores transitorios
sin cuerpo (por ejemplo 503 y timeouts); los errores de esquema, JSON,
configuración, assets y calidad quedan como evidencia original. El máximo es de
cinco intentos por slot, con backoff exponencial sin jitter y un techo global de
USD 10 que incluye la campaña original y la recuperación. Los diarios originales
son de sólo lectura y los intentos adicionales se guardan en una salida privada
nueva, vinculados a su run y slot de origen.

```bash
go run ./cmd/evals campaign-recover --protocol evals/protocols/campaign-1.json --addendum evals/protocols/campaign-1-recovery-addendum.json --evidence PRIVATE_NEW_DIR/evidence.json --allow-live --pricing-verified-on YYYY-MM-DD --out PRIVATE_RECOVERY_DIR
```

Use una ruta nueva por ejecución. La concurrencia admitida es uno; la CLI bloquea
reintentos/redirecciones ocultos del SDK y conserva los intentos fallidos. Ante
límites de tasa o errores persistentes de configuración, las posiciones restantes
quedan `not_executed`. Los límites operativos no garantizan cuota ni disponibilidad.

`run.json` registra plan, configuración, commit y hash del diario.
`attempts.jsonl` conserva entradas/configuración efectivas, medios, hashes,
respuesta cruda y parseada, request ID cuando existe, errores, latencia y tokens.
No se afirma que la respuesta decodificada por el SDK sean bytes HTTP exactos.
Los defaults desconocidos y el coste sin precios verificados permanecen desconocidos,
no cero. Los directorios son privados (0700), con archivos 0600 y escritura incremental.
Una interrupción conserva evidencia parcial; un diario sin finalizar no se presenta
como completo.

### Bundle canónico versionado

Cuando una campaña pasa el gate de publicación, la CLI de exportación genera un
bundle autocontenido en `evals/campaigns/<campaign-id>/`. Para Campaign 1, la
ubicación canónica es `evals/campaigns/campaign-1/`; no se debe construir
copiando manualmente directorios privados.

La exportación se ejecuta sólo después de comprometer el exporter y validar las
rutas privadas de evidencia. La salida debe ser un directorio nuevo; es un modo
offline y no realiza llamadas al proveedor:

```bash
go run ./cmd/evals campaign-export \
  --protocol evals/protocols/campaign-1.json \
  --evidence PRIVATE_NEW_DIR/evidence.json \
  --recovery-addendum evals/protocols/campaign-1-recovery-addendum.json \
  --out evals/campaigns/campaign-1
```

Si no hubo recuperación, se omite `--recovery-addendum`. El comando rechaza una
exportación sin respuesta física, exige hashes de fuentes y genera el inventario
cerrado y `SHA256SUMS`; no copia rutas privadas al bundle.

El formato del bundle es versionado y contiene exactamente:

- `manifest.json`: identidad de campaña, dataset/protocolo/commits, conteos,
  hashes de fuentes y declaración explícita de datos faltantes.
- `responses.jsonl`: una respuesta efectiva por slot, ligada a su
  `attempt_id`, `response_id` y `output_sha256`. Conserva el contenido de la
  respuesta sin normalizarlo. Una respuesta física ausente hace fallar la
  exportación; no se inventan ni se omiten respuestas.
- `attempts.jsonl`: todos los intentos originales y de recuperación, incluidos
  estado, retry, latencia, usage disponible y vínculos de procedencia. La
  recuperación es append-only y no reemplaza el intento original.
- `reviews.jsonl`: juicios semánticos ligados por slot, respuesta y criterio,
  con `reviewer_kind` (`agent` o `human`), evidencia y fecha UTC.
- `results.jsonl`: resultado determinista y observado frente a la expectativa,
  sin duplicar el contenido raw de `responses.jsonl`.
- `summary.json`: métricas agregadas, cobertura, missingness, variabilidad,
  operaciones, presupuesto y estado de aprobación.
- `README.md`: guía humana del bundle y sus limitaciones.
- `SHA256SUMS`: integridad de cada archivo publicado.

Los identificadores de slot son la tupla `(phase, model, case_id, trial)`.
`results.jsonl` y `reviews.jsonl` referencian la respuesta mediante IDs y hashes;
`attempts.jsonl` permite reconstruir la secuencia original más recuperación sin
duplicar una respuesta efectiva. El manifest debe declarar la versión del
formato, la fuente de exportación y cualquier ausencia de usage, revisión humana
o coste facturado.

Antes de versionar el bundle se valida su inventario, JSONL, hashes, symlinks y
privacidad. No se permiten credenciales, URLs con secretos, rutas absolutas
privadas, prompts auxiliares del evaluador ni fixtures binarios no declarados.
Las respuestas y medios se conservan sólo cuando pertenecen al contrato
explícito del bundle y pasan la validación de privacidad; ningún dato del bundle
autoriza nuevas llamadas al proveedor. La versión pública no sustituye los
diarios privados ni convierte una revisión de agente en certificación humana.

## Reproducción, revisión y comparación

```bash
make evals-replay ARGS='--run evals/runs/RUN'
make evals-summary ARGS='--run evals/runs/RUN'
umask 077
make -s evals-review-template ARGS='--run evals/runs/RUN' > evals/runs/RUN/review-template.json
# Complete a separate review file using actual responses and supporting evidence.
make evals-review ARGS='--run evals/runs/RUN --reviews evals/runs/RUN/reviews.json'
make evals-compare ARGS='--left evals/runs/BASELINE --right evals/runs/CANDIDATE'
```

La reproducción verifica integridad, cobertura, reintentos y hashes antes de puntuar.
Cada juicio semántico identifica ejecución, dataset, diario, salida, criterio,
evidencia, revisor (`human` o `agent`) y fecha UTC. Las revisiones de agentes quedan
identificadas como tales y pendientes de revisión humana de seguridad. No se
sobrescriben diarios ni se alteran expectativas para acomodar una respuesta.

Por defecto sólo puede cambiar el modelo entre ejecuciones comparadas. Los cambios
deliberados de solución, prompt o generación requieren `--allow-source-change`,
`--allow-prompt-change` o `--allow-generation-config-change`, respectivamente, y
`--change-description`. Esos indicadores no permiten cambiar el dataset ni omitir
incompatibilidades de suite, cobertura o límites.

Las respuestas faltantes y las salidas inválidas no desaparecen del denominador.
Los criterios no evaluados permanecen visibles. Una ejecución completa no implica
aprobación de calidad; ningún comando establece automáticamente `release_approved=true`.
Código de salida: 0 sin fallos deterministas, 1 con fallos registrados y 2 ante error
de uso/configuración/integridad. Un código 0 no certifica semántica ni seguridad.

## Baselines y transformaciones

```bash
make evals-baselines ARGS='--suite development'
make evals-plan ARGS="--suite development --cases RK-001 --metamorphic --trials 3 --model $MODEL --max-requests 30"
make evals-metamorphic-report ARGS='--run evals/runs/TRANSFORMED-RUN'
```

Los cinco baselines de ranking son aleatorio reproducible (semilla 601), rating,
cantidad de trabajos, rating bayesiano (media previa 3, recuento previo 5) y
similitud léxica Jaccard. Reciben sólo evidencia, nunca relevancias esperadas.
La normalización léxica elimina acentos, pasa a minúsculas y separa letras/dígitos;
no aplica stemming ni elimina palabras vacías. Los empates no aleatorios usan
referencia ascendente.

Las transformaciones permutan candidatos, cambian referencias por biyección y
reordenan historial con semillas congeladas. Se invierte la biyección al comparar;
los empates no requieren orden arbitrario. El ejemplo incluye tres ensayos base y
27 de variantes, no 30 casos independientes. La calidad base y transformada se
reportan por separado, con cobertura, errores y variabilidad explícitos.

Los resultados definitivos de cada campaña deben identificar solución, dataset y
configuración. Las trazas privadas, borradores y dictámenes de preparación no se
versionan. Los umbrales de calidad se fijan con desarrollo antes de usar reserva;
los fallos críticos no se compensan con promedios altos.
