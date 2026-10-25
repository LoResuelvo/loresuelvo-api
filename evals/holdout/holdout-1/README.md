# Holdout

Resultados de los 19 casos de reserva con `gemini-3.5-flash-lite` y tres trials por caso: **57 observaciones** (14 casos de prediagnóstico y cinco de ranking). La solución evaluada es `41dfefa`.

## Resultado

El criterio de aprobación exige que todas las comprobaciones aplicables aprueben. Cumplen **41 de 57 respuestas (71,9 %)**. Los 366 criterios semánticos contienen 350 `pass`, 16 `fail` y 0 `not_applicable`, sin criterios pendientes. Las comprobaciones deterministas pasan en 52 observaciones y fallan en 5. El holdout no aprueba.

La revisión final de los 19 casos y sus 57 respuestas está completa. Los resultados incorporan la revisión contextual de los juicios y sus evidencias, con la misma solución, los mismos criterios y las mismas condiciones de aprobación.

Se obtuvieron las 57 respuestas con 58 intentos físicos de generación, incluido uno sin respuesta, y 30 solicitudes auxiliares de conteo de tokens. Las respuestas recibidas se conservaron sin repetirlas. El manifiesto distingue los dos tramos de adquisición y sus ejecutables; los archivos de la solución coinciden con el candidato seleccionado.

## Archivos

| Archivo | Contenido |
| --- | --- |
| `responses.jsonl` | 57 salidas originales y estructuradas, hashes y datos de generación. |
| `results.jsonl` | 57 resultados con expectativas, observaciones, métricas y comprobaciones. |
| `evaluations.jsonl` | 366 criterios con resultados, razones y evidencia. |
| `summary.json` | Agregados por tarea y modelo, cobertura y aprobación. |
| `manifest.json` | Dataset, solución, configuraciones y hashes de los archivos. |
| `SHA256SUMS` | Hashes de los otros seis archivos. |

`slot_id` y `response_id` vinculan los archivos. `output_sha256` identifica los bytes UTF-8 de `raw_output` sin reformatearlos. Cada evaluación conserva el hash de su criterio y citas literales o la verificación de una ausencia. Las métricas promedian primero los trials de cada caso y después los casos; sus contadores indican la cobertura disponible. Los tiempos y tokens corresponden a la generación seleccionada. Un dato no informado permanece `null` con su motivo; el uso no representa un importe facturado.

## Integridad

Desde esta carpeta:

```bash
sha256sum --check SHA256SUMS
```

Dataset: [`LoResuelvo_US60_evals_v1.0.0`](../../datasets/LoResuelvo_US60_evals_v1.0.0/).
