# Integración de la evaluación US-60

Estado: contratos sin conexión, validación local de esquemas JSON y puntuación determinista,
ejecución en vivo acotada, diarios incrementales, reproducción y comparación pareada de modelos
están implementados, incluidas transformaciones metamórficas explícitas, cinco líneas base no LLM,
resúmenes agregados e importación de revisiones semánticas vinculadas a procedencia.
La aprobación de seguridad del modelo es independiente y nunca se infiere de la finalización de la herramienta.
Véase la [evidencia de la medición inicial y la finalización](reports/2026-09-08-us60-completion.md):
la herramienta fue entregada, pero los fallos de seguridad observados en el modelo impiden la aprobación.

## Campañas completas publicadas

- [US-60 — solución `76f683e`, dataset `1.1.0`](reports/2026-09-08-us60-solution-76f683e-dataset-1.1.0/README.md):
  informe narrativo y [resultados por trial](reports/2026-09-08-us60-solution-76f683e-dataset-1.1.0/results.json),
  medidos contra el [dataset numérico `1.1.0`](datasets/LoResuelvo_US60_evals_v1.1.0/README.md).

Cada campaña completa se publica en un directorio propio identificado por la solución
y la versión del dataset; no se combinan resultados de otras soluciones o datasets.

## Fuente e integridad

- Especificación: https://github.com/LoResuelvo/loresuelvo-api/issues/205
- Archivo original: `incoming/LoResuelvo_US60_evals_v1.0.0.zip` (local, ignorado).
- SHA-256 del archivo: `f253e81a18af481f0b91fad3222ac3c7f6de1e9d985c9c8a971eadfcaf276808`.
- Documentación mantenida: [`DATASET.md`](DATASET.md).
- Paquete de referencia: `datasets/LoResuelvo_US60_evals_v1.0.0/` (copia histórica inmutable del paquete canónico).
- Commit de backend de referencia: `2a7f77fda6c6175753f73e095de0a0bd19169a06`.
- Commit de importación: `ed787f6e794e8414a5182fce4ea1d67d75876453`.

El 2026-09-08, el validador incluido y las 47 pruebas de Python pasaron:
48 casos de prediagnóstico, 24 de ranking, 12 especificaciones de contrato, ocho PNG,
54 casos de desarrollo, 18 de reserva, 18 smoke y 13 críticos.
Estas comprobaciones no ejecutaron los comportamientos CT ni llamaron a un modelo.

## Reproducir las comprobaciones del paquete

Desde la raíz del repositorio, usando Python 3.10 o posterior:

```bash
python3 -m venv evals/.venv
evals/.venv/bin/python -m pip install -r evals/datasets/LoResuelvo_US60_evals_v1.0.0/requirements.txt
PYTHONDONTWRITEBYTECODE=1 evals/.venv/bin/python evals/datasets/LoResuelvo_US60_evals_v1.0.0/tools/evalpack.py validate
PYTHONDONTWRITEBYTECODE=1 evals/.venv/bin/python -m unittest discover -s evals/datasets/LoResuelvo_US60_evals_v1.0.0/tests -v
```

La instalación de dependencias puede requerir acceso de red. Las comprobaciones no requieren credenciales ni red. No regenere manifiestos
para resolver una discrepancia.

## Organización de la implementación

- `cmd/evals/`: composición y autorización explícitas de la CLI.
- `internal/evals/`: mapeo de datos, planes, arnés de contratos y límites de ejecución,
  puntuación, transformaciones, persistencia de resultados, reproducción y comparación.
- `internal/adapters/chatbot/`: configuración mínima de generación y soporte de observación,
  reutilizando sin cambios los prompts y analizadores de producción.
- `evals/configs/`: configuración experimental derivada, fuera de los datos congelados.
- `evals/runs/<run_id>/`: trazas locales saneadas, revisiones semánticas e informes.

La línea base inicial está registrada en el informe enlazado arriba. Las correcciones deben compararse con esa evidencia; la reserva se mantiene fuera del ajuste. CT-08 y CT-10
necesitan evidencia estructural y semántica separadas: una respuesta simulada no puede demostrar
detección real de riesgos, orientación segura ni preservación de hechos por el modelo.

## Preparación offline en Go

Desde la raíz del repositorio:

```bash
make evals-validate
make evals-plan ARGS='--suite smoke --model gemini-3.5-flash-lite --max-requests 18'
make evals-plan ARGS='--suite development --model gemini-3.5-flash-lite --max-requests 162'
```

Estos comandos nunca crean un cliente de modelo, cargan credenciales ni ejecutan comportamientos CT. La validación Go comprueba la integridad de archivos, los esquemas Draft 2020-12 locales y las referencias de casos/suites
y el mapeo de dominio; use la validación Python anterior para las comprobaciones adicionales
de consistencia editorial y de políticas congeladas. Una comprobación de integridad aprobada no equivale a un experimento aprobado.

Los valores predeterminados de los ensayos provienen de `configs/experiment.json`: la prueba smoke, la línea base de desarrollo
y la validación de lanzamiento para las suites de reserva/críticas. `--trials` anula explícitamente el recuento.
El límite de solicitudes incluye cada reintento permitido (`--max-retries`, cero por defecto).
Los planes que exceden ese límite se rechazan en vez de truncarse silenciosamente.
`--allow-holdout` es obligatorio para reserva o critical_all y para cualquier selección
que contenga casos de reserva. Solo autoriza la planificación, no llamadas en vivo.

La vista previa base cubre solo ejecuciones PD/RK, sin CT, transformaciones ni autorización en vivo implícitas. El modelo es metadato solicitado;
no se afirma configuración efectiva ni precio monetario. El costo permanece en null.

## Ejecución y evidencia

Las comprobaciones de contratos son manuales y offline:

```bash
make evals-contract
```

Las doce especificaciones tienen ejecutores. CT-02 valida el límite servicio/repositorio, pero no puede establecer filtrado SQL real; CT-08 y CT-10 no pueden certificar detección de riesgos ni semántica de resumen con respuestas controladas.
Se informan como `unassessed`, no aprobadas. CT-09 ejercita persistencia de tiempos de espera y puntuación real de reproducción. Los prompts de producción y el comportamiento de dominio no cambian.

Antes de la ejecución en vivo, exporte `CHATBOT_API_KEY` mediante su mecanismo local seguro de credenciales. El ejecutor no carga `.env`, y la sola presencia de una clave nunca autoriza la generación. No incluya credenciales en argumentos,
commits, informes ni historial de shell. El modelo es explícito, no heredado de una configuración mutable de despliegue.

Vista previa sin credenciales ni red:

```bash
make evals-live ARGS='--dry-run --suite smoke --model gemini-3.5-flash-lite --max-requests 18 --attempt-timeout 90s --global-timeout 15m --min-interval 15s --max-output-tokens 4096'
```

Tras autorizar, desde un árbol de trabajo limpio y versionado:

```bash
make evals-live ARGS='--allow-live --suite smoke --model gemini-3.5-flash-lite --max-requests 18 --attempt-timeout 90s --global-timeout 15m --min-interval 15s --max-output-tokens 4096 --out evals/runs/smoke-3.5-UNIQUE'
```

Use un directorio de salida nuevo cada vez. Para comparar 3.1, use el mismo commit y
configuración con `--model gemini-3.1-flash-lite` y otro directorio. Cada invocación
tiene su propia autorización y límite estricto de solicitudes. La implementación inicial
admite concurrencia 1; los reintentos son cero por defecto. Bloquea reintentos/redirecciones del SDK,
se detiene ante limitación de tasa o fallos persistentes de configuración del proveedor y registra
los casos restantes como `not_executed`. El intervalo es un límite operativo, no una afirmación
sobre la cuota RPM/TPM/RPD real de la cuenta. Verifique la cuota disponible en AI Studio.

Cada intento se sincroniza antes de continuar. `run.json` registra el plan/configuración, el commit y la suma de comprobación del diario; `attempts.jsonl` conserva el texto bruto del modelo y la salida de dominio analizada, las entradas/configuración reales del SDK, bytes multimedia, hashes e ID de solicitud cuando se proporciona
y metadatos del proveedor. La respuesta del proveedor decodificada por el SDK no se afirma como bytes HTTP exactos;
no se persisten secretos ni encabezados HTTP. Los valores predeterminados no especificados siguen siendo desconocidos.
Los archivos son locales, modo 0600, dentro de un directorio modo 0700. Los límites SIGINT/tiempo conservan resultados parciales;
un diario sin finalizar por una terminación abrupta permanece disponible, pero la reproducción se niega a presentarlo como completo.

```bash
make evals-replay ARGS='--run evals/runs/smoke-3.5-UNIQUE'
make evals-compare ARGS='--left evals/runs/smoke-3.1-UNIQUE --right evals/runs/smoke-3.5-UNIQUE'
```

La reproducción verifica integridad, cobertura de casos/ensayos, reintentos y hashes de entrada/prompt antes de puntuar respuestas históricas. Por defecto, la comparación solo permite cambiar el modelo: deben coincidir conjunto de datos, commit, suite, ensayos, configuración de generación y entradas pareadas. Los cambios deliberados de origen, prompt/entrada o generación requieren el indicador correspondiente `--allow-source-change`, `--allow-prompt-change` o
el indicador `--allow-generation-config-change` y una `--change-description` no vacía.
Las diferencias observadas se informan; estos indicadores nunca relajan el conjunto de datos ni la suite,
cobertura de casos/ensayos, presupuesto de solicitudes ni compatibilidad de límites operativos.
Las respuestas faltantes, las salidas no válidas y los criterios semánticos no evaluados siguen visibles.
No existe una ruta automática de `release_approved=true`. El costo permanece en null sin precios verificados. Los resúmenes agregados son JSON; los informes Markdown concisos versionados enlazan evidencia local.

Códigos de salida: 0 significa que el comando terminó sin fallos deterministas, 1 significa fallos de ejecución/contrato/deterministas registrados y 2 significa fallo de uso, configuración o integridad. El código 0 nunca certifica semántica ni seguridad. Los comandos de prueba no ejecutan esta batería experimental ni realizan llamadas al modelo.


## Líneas base, transformaciones y revisión

Todos los comandos siguientes son offline salvo que se etiqueten explícitamente como `live`. Las pruebas ordinarias continúan usando fixtures sintéticos; ninguna ejecuta esta batería experimental.

```bash
make evals-baselines ARGS='--suite development'
make evals-summary ARGS='--run evals/runs/RUN'
umask 077 # protect redirected local review files
make -s evals-review-template ARGS='--run evals/runs/RUN' > evals/runs/RUN/review-template.json
# Copy to a new review file, assess actual responses and fill evidence/provenance.
make evals-review ARGS='--run evals/runs/RUN --reviews evals/runs/RUN/reviews-v1.json'
```

Los documentos de revisión vinculan el ID de ejecución, el conjunto de datos y los hashes del diario de intentos, cada hash de salida sin procesar y el hash del criterio. Cada juicio sobre una respuesta concreta requiere evidencia (fragmento y justificación), identidad declarada del revisor, tipo de revisor (`human` o `agent`) y marca UTC. Se revisa la respuesta frente al criterio y la entrada del caso, no se vuelve a aprobar el dataset. El tipo identifica quién hizo la revisión; no acredita especialización. Las respuestas faltantes no pueden recibir evaluaciones positivas; las entradas desconocidas, duplicadas u obsoletas se rechazan.
Los juicios del agente siguen identificados visiblemente como redactados por un agente y pendientes de revisión humana de seguridad.
Ninguna importación puede establecer `release_approved=true`. Las revisiones se realizan en archivos separados;
no se editan los diarios originales ni el conjunto de datos congelado.

Las políticas de ranking están congeladas en el código y serializadas en el informe de línea base:
aleatorio con semilla (601), promedio de calificación, cantidad de trabajos pagados, calificación bayesiana (media previa 3, recuento previo 5) y Jaccard de conjuntos de tokens léxicos. La normalización léxica pasa a minúsculas, elimina acentos y separa letras/dígitos Unicode; no usa palabras vacías ni stemming.
La consulta usa título/descripción del problema; el texto candidato usa descripciones de trabajos, informes de finalización y revisiones. Los empates no aleatorios usan referencia ascendente.
Los algoritmos reciben solo evidencia de entrada elegible, nunca la relevancia esperada.

Los resúmenes muestran ponderación de casos base, reintentos terminales, cobertura completa de ensayos, fallos técnicos, subtotales observados de latencia/tokens y cobertura desconocida. La calidad base y transformada se informan por separado. No compare variantes mixtas con líneas base solo base ni trate ensayos/familias repetidos como
muestras de producción independientes. El costo permanece en null sin precios verificados.

Vista previa explícita de transformación para un caso de desarrollo seleccionado:

```bash
make evals-plan ARGS='--suite development --cases RK-001 --metamorphic --trials 3 --model gemini-3.5-flash-lite --max-requests 30'
```

Esto incluye tres ensayos base más tres transformaciones × tres semillas × tres ensayos: 30 solicitudes, no 30 casos independientes. `--cases` solo puede estrechar la suite nombrada. `--metamorphic` usa semillas congeladas y recuentos exactos. No se selecciona implícitamente contraparte, caso de reserva ni reintento. El plan completo de transformación de desarrollo tiene 486 intentos de variantes más 162 intentos base y no debe confundirse con una ejecución diaria segura para el presupuesto bajo una cuota de 500 solicitudes.

Tras la autorización explícita, use ese plan con `live`, `--allow-live`, todos los indicadores de límite finito y un directorio de salida nuevo. Por ejemplo, una ejecución de 30 solicitudes puede utilizar `--attempt-timeout 90s --global-timeout 30m --min-interval 5s
--max-output-tokens 4096`. El intervalo permite como máximo 12 inicios de solicitudes por minuto
por invocación; las invocaciones concurrentes y otras aplicaciones comparten cuotas del proveedor y deben presupuestarse juntas.
Una cuota mostrada no garantiza disponibilidad del proveedor. HTTP 503 sigue siendo un fallo técnico, no un fallo de calidad.

```bash
make evals-metamorphic-report ARGS='--run evals/runs/TRANSFORMED-RUN'
```

La entrada/salida transformada original permanece en el diario. La puntuación reconstruye e invierte biyecciones de referencia; el informe empareja ensayos coincidentes y registra semilla, padre y división. La pertenencia al top tres se compara ignorando empates, mientras el orden estricto se comprueba solo mediante restricciones pareadas explícitas. Las transformaciones sin efecto y las salidas faltantes/no válidas siguen sin evaluar. Los pares PD congelados pueden compararse desde cualquier ejecución que contenga ambos miembros; los ausentes permanecen visibles y nunca se añaden implícitamente. Las decisiones coincidentes no certifican resistencia a inyección.

Una línea base medida que falla es un hallazgo válido. Cerrar US-60 no aprueba un lanzamiento del modelo, no demuestra riesgo cero, exactitud con fotos reales ni reducción del esfuerzo del usuario. El uso de la reserva sigue siendo una operación separada y consciente después de congelar las decisiones de desarrollo; los casos críticos de reserva no ejecutados siguen sin evaluar e impiden la aprobación del lanzamiento.


Una nueva línea base de desarrollo de tres ensayos, autorizada explícitamente, puede reproducirse con los siguientes límites finitos (compruebe primero la cuota restante de la cuenta):

```bash
make evals-live ARGS='--allow-live --suite development --trials 3 --model gemini-3.5-flash-lite --max-requests 162 --max-retries 0 --attempt-timeout 90s --global-timeout 90m --min-interval 5s --max-output-tokens 4096 --out evals/runs/development-3.5-UNIQUE'
```

Los [umbrales derivados iniciales](configs/initial-development-v1.json) son comprobaciones manuales exploratorias de no regresión, no compuertas automáticas de lanzamiento ni calidad de producto aceptable. La configuración se copia junto a las ejecuciones de origen y permanece separada del corpus congelado. Deben inspeccionarse tanto la cobertura numérica como la cobertura real de salidas críticas revisables; los errores nunca se convierten en evidencia positiva de seguridad.

## Extensión visual derivada

La selección visual sintética derivada (cuatro casos) está en [`datasets/LoResuelvo_US60_visual_v1.0.0/`](datasets/LoResuelvo_US60_visual_v1.0.0/).
