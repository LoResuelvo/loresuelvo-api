# Campaña 2

Resultados de los 52 casos de desarrollo con `gemini-3.1-flash-lite` y
`gemini-3.5-flash-lite`, con tres trials por caso y modelo: **312 observaciones**,
156 por modelo. Incluye 34 casos de prediagnóstico, 14 de ranking y cuatro de
resumen. La solución evaluada es `6a9a030`; el manifiesto identifica el dataset,
los archivos de la solución y las configuraciones de generación.

## Resultados

Los **2082 criterios semánticos** están evaluados: 1910 `pass`, 168 `fail` y
cuatro `not_applicable`. Cumplen todas las comprobaciones aplicables 191
respuestas; 121 tienen algún fallo. Las comprobaciones deterministas pasan en
292 observaciones y fallan en 20.

Se conservan las 312 respuestas recibidas, incluida una con formato inválido y
estado `execution_error`. Esa respuesta mantiene su fallo determinista y sus
ocho resultados semánticos. Los resultados no se excluyen por fallar una
comprobación.

## Archivos

| Archivo | Contenido |
| --- | --- |
| `responses.jsonl` | 312 respuestas, salida original y estructurada cuando existe, hashes y datos de generación. |
| `results.jsonl` | 312 resultados efectivos con expectativas, observaciones, métricas y comprobaciones. |
| `evaluations.jsonl` | 2082 criterios con resultados, razones y evidencia. |
| `summary.json` | Agregados por tarea y modelo, métricas y cobertura de los datos de generación. |
| `manifest.json` | Identidad de la solución, dataset y configuraciones; inventario con hashes y tamaños. |
| `SHA256SUMS` | Hashes de los otros seis archivos. |

`slot_id` y `response_id` vinculan los archivos. `output_sha256` identifica los
bytes UTF-8 de `raw_output` sin reformatearlos; cada evaluación conserva el hash
de su criterio. `evidence` contiene las citas literales o la verificación de una
ausencia, y `reviewed_fields` delimita la prosa evaluada cuando corresponde.

Las métricas promedian primero los trials de cada caso y después los casos.
Sus contadores indican la cobertura disponible. Pairwise abarca los doce casos
de ranking con restricciones de pares. Los tiempos y tokens corresponden a la
generación seleccionada; un dato no informado permanece `null` con su motivo.
Los tokens de entrada ya incluyen los cacheados y el total incluye entrada,
salida y pensamiento. Los datos de uso no representan un importe facturado.

## Integridad

Desde esta carpeta:

```bash
sha256sum --check SHA256SUMS
```

El dataset está en
[`LoResuelvo_US60_evals_v1.0.0`](../../datasets/LoResuelvo_US60_evals_v1.0.0/).
