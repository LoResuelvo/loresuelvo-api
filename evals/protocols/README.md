# Protocolo de la primera campaña

[`campaign-1.json`](campaign-1.json) fija el dataset, la solución por commit y
hashes, los modelos, las repeticiones, los límites y las tarifas de referencia.
Definirlo no ejecuta Gemini ni aprueba sus respuestas.

## Plan y distribución

- Golden aprobado: 70 casos; sólo se ejecutan los 51 de desarrollo.
- Modelos: `gemini-3.1-flash-lite` y `gemini-3.5-flash-lite`.
- Smoke de campaña: 17 casos × 1 intento × 2 modelos = 34 generaciones.
- Desarrollo: 51 casos × 3 intentos × 2 modelos = 306 generaciones.
- Total: como máximo 340 generaciones y 136 solicitudes auxiliares CountTokens.
  No se ejecutan reserva, transformaciones ni reintentos de calidad.

`CHATBOT_EVALS_API_KEYS` contiene las claves separadas por comas, configuradas
privadamente. Cada clave debe corresponder a un proyecto independiente. El
runner admite N claves; este protocolo permite hasta tres ejecuciones simultáneas.
Una cola distribuye los slots caso/trial entre proyectos disponibles: no divide
el dataset en tres subconjuntos fijos ni ejecuta cada caso una vez por clave.
Ambos modelos reciben los mismos casos y repeticiones. La asignación concreta
puede variar con disponibilidad y duración; no cambia los denominadores.
El pacing mínimo de 15 segundos se aplica por proyecto y se comparte entre
modelos, fases y CountTokens. Un proyecto detenido no recibe nuevos slots.

Los límites de cuota efectivos deben verificarse en AI Studio para cada
proyecto antes de ejecutar; tres claves no garantizan capacidad ni duración.
Ver [cuotas por proyecto](https://ai.google.dev/gemini-api/docs/rate-limits).

## Preparación offline

Desde la raíz del repositorio, con código versionado y checkout limpio:

```bash
mkdir -p tmp
go build -o tmp/evals-runner ./cmd/evals
tmp/evals-runner validate
tmp/evals-runner plan --suite development --model gemini-3.1-flash-lite --trials 3 --max-requests 153
tmp/evals-runner plan --suite development --model gemini-3.5-flash-lite --trials 3 --max-requests 153
tmp/evals-runner campaign-live --protocol evals/protocols/campaign-1.json --dry-run
```

Previsualizar también el smoke independiente para cada modelo, sustituyendo
`MODEL` por uno de los dos identificadores del protocolo:

```bash
tmp/evals-runner live --suite smoke --model MODEL --trials 1 --max-requests 17 \
  --max-retries 0 --concurrency 3 --attempt-timeout 90s --global-timeout 16200s \
  --min-interval 15s --max-output-tokens 4096 --billing-tier free --dry-run
```

Estos comandos no llaman al proveedor. `solution.baseline_source_commit` identifica
la revisión que contiene las fuentes fijadas por hash. Una revisión posterior puede
añadir este protocolo y su documentación sin alterar esas fuentes. El build debe
conservar metadata VCS de la revisión efectiva y limpia de ejecución; los hashes
de `solution.baseline_files` deben seguir coincidiendo con la base. Una modificación
de la solución requiere otro protocolo explícito, no actualizar silenciosamente la base.

## Smoke independiente: autorización e inspección

**No ejecutar hasta autorizar llamadas reales.** El ejecutor `campaign-live`
realiza todo CountTokens antes de generar y luego corre smoke y desarrollo sin
pausa de aprobación. Por eso la comprobación previa es una corrida independiente
por modelo; no es un gate automático dentro de `campaign-live`.

Para cada modelo, en invocaciones separadas y directorios nuevos:

```bash
mkdir -p tmp/evals-campaign-1/precheck
tmp/evals-runner live --suite smoke --model MODEL --trials 1 --max-requests 17 \
  --max-retries 0 --concurrency 3 --attempt-timeout 90s --global-timeout 16200s \
  --min-interval 15s --max-output-tokens 4096 --billing-tier free \
  --allow-live --out tmp/evals-campaign-1/precheck/MODEL
```

Inspeccionar el journal y las salidas: `MAX_TOKENS`, truncamiento, formato,
respuestas vacías, metadata ausente, errores y 429. Verificar disponibilidad de
ambos modelos y suficiencia del límite antes de autorizar desarrollo. Si falla,
detenerse y resolver explícitamente; no elevar límites ni cambiar prompts a mitad
de medición. Este smoke no acredita calidad semántica ni seguridad general.

Estas 34 generaciones adicionales quedan **fuera** del techo de 340 de campaña.
`live` tiene límites operativos finitos, pero no la guardia monetaria de
`campaign-live`. Conservar su evidencia separada; no confundirla con las fases
medidas de campaña ni ocultar su consumo al calcular el total de preparación.

## Ejecución de campaña: autorización separada

**Sólo tras aceptar el smoke independiente.** Verificar de nuevo las
[tarifas oficiales](https://ai.google.dev/gemini-api/docs/pricing) en la fecha UTC
real de ejecución. El snapshot actual está verificado el 2026-10-13; si cambia la
fecha o tarifa, actualizar y versionar el snapshot antes de congelar la ejecución.
No sustituir la fecha automáticamente por la del sistema sin verificar precios.

```bash
mkdir -p tmp/evals-campaign-1
tmp/evals-runner campaign-live --protocol evals/protocols/campaign-1.json \
  --pricing-verified-on FECHA_UTC_VERIFICADA --allow-live \
  --out tmp/evals-campaign-1/measurement
```

El directorio final debe ser nuevo. La guardia reserva cada solicitud antes del
transporte con un techo nominal de referencia de USD 10. La cota conservadora
es USD 5,5216 (340 × [20.000 × 0,30 + 4.096 × 2,50] / 1.000.000), con USD 4,4784
de margen. No representa cargos: se usa cuota gratuita y el importe facturado
permanece desconocido. El costo se calcula externamente con uso nativo y tarifas
estándar TEXT/IMAGE; salida incluye pensamiento y caché es un subconjunto de
entrada. No se pide al modelo devolver costos. CountTokens se cuenta aparte,
sin precio auxiliar inventado ni ledger individual de latencia.

## Lectura y publicación de resultados

Las siguientes operaciones son offline, posteriores a las corridas:

```bash
tmp/evals-runner baselines --suite development
tmp/evals-runner replay --run RUTA_CORRIDA
tmp/evals-runner summary --run RUTA_CORRIDA
tmp/evals-runner campaign-report --protocol evals/protocols/campaign-1.json \
  --evidence tmp/evals-campaign-1/measurement/evidence.json \
  --out tmp/evals-campaign-1/report
tmp/evals-runner campaign-export --protocol evals/protocols/campaign-1.json \
  --evidence tmp/evals-campaign-1/measurement/evidence.json \
  --out tmp/evals-campaign-1/export
```

Evaluar las salidas semánticamente por criterio antes de obtener conclusiones;
los resultados sin evaluar quedan `unassessed`. El export conserva todos los
intentos físicos y respuestas, con un resultado efectivo por slot. Sólo se
publica tras verificar integridad y privacidad. No contiene identidades ni
razonamientos privados de revisión. No inferir aprobación de release ni riesgo
cero; repeticiones y smoke superpuesto no son observaciones independientes.

No se crea una recuperación anticipada. Un fallo técnico real requiere selección
trazable y autorización expresa; no sustituir una respuesta por otra mejor.
