# Campaña 3

Resultados de los 52 casos de desarrollo con `gemini-3.1-flash-lite` y
`gemini-3.5-flash-lite`, con tres trials por caso y modelo: **312 observaciones**,
156 por modelo. Comprende 34 casos de prediagnóstico, 14 de ranking y cuatro de
resumen. La solución evaluada es `41dfefa`; el manifiesto identifica el dataset,
los archivos de la solución y las configuraciones de generación.

## Resultados

Los **2082 criterios semánticos** están evaluados: 1923 `pass`, 154 `fail` y
cinco `not_applicable`. Cumplen todas las comprobaciones aplicables **208
respuestas**; 104 tienen algún fallo. Las comprobaciones deterministas pasan en
305 observaciones y fallan en siete.

Las 312 observaciones tienen una respuesta recibida y evaluada. Se registraron
312 intentos originales, incluidos ocho errores HTTP 503 sin respuesta,
y ocho intentos de recuperación que sí recibieron salida: **320 solicitudes
físicas para 312 observaciones efectivas**.

## Archivos

| Archivo | Contenido |
| --- | --- |
| `responses.jsonl` | 312 respuestas efectivas, salida original y estructurada, hashes y datos de generación. |
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
Sus contadores indican la cobertura disponible. Los tiempos y tokens
corresponden a la generación seleccionada; un dato no informado permanece
`null` con su motivo. Los datos de uso no representan un importe facturado.

El contraste con C2 y las conclusiones por comportamiento están en el
[diagnóstico de Campaña 3](../../analysis/campaign-3-diagnosis.md).

## Integridad

Desde esta carpeta:

```bash
sha256sum --check SHA256SUMS
```

El dataset está en
[`LoResuelvo_US60_evals_v1.0.0`](../../datasets/LoResuelvo_US60_evals_v1.0.0/).
