# Campaña 1

Resultados de los 52 casos de desarrollo, evaluados con `gemini-3.1-flash-lite`
y `gemini-3.5-flash-lite`, con tres trials por caso y modelo: **312 observaciones**,
156 por modelo. La muestra contiene 34 casos de prediagnóstico, 14 de ranking
y 4 de resumen. El dataset y las configuraciones se identifican en el manifiesto.

## Calidad

Los 2082 juicios están evaluados: 1692 `pass` y 390 `fail`. Los resultados finales
son 96 `semantic_reviewed` y 216 `failed`; 289 observaciones cumplen las
comprobaciones deterministas y 23 fallan. La revisión de las 312 observaciones
está completa. La evaluación conserva criterios, justificaciones y evidencia
por respuesta; completar la revisión no convierte un fallo en un acierto.

`summary.json` reúne los resultados por modelo y tarea, las métricas de calidad
y los datos de generación de las respuestas seleccionadas. Las medias de calidad
promedian primero los trials de cada caso y después los casos; los contadores
indican las observaciones y casos con un valor disponible para cada métrica.
La comparación de ranking incluye cinco políticas offline sobre los mismos
14 casos. Las métricas pairwise corresponden a los 12 casos con restricciones
de ese tipo; su ausencia en los otros dos casos no representa un juicio pendiente.

## Archivos

| Archivo | Contenido |
| --- | --- |
| `responses.jsonl` | 312 respuestas con su salida original, salida estructurada y datos de generación. |
| `results.jsonl` | 312 resultados con expectativas, observaciones, métricas y estados de calidad. |
| `evaluations.jsonl` | 2082 juicios con criterio, justificación y evidencia. |
| `summary.json` | Agregados por modelo y tarea, métricas y políticas de comparación. |
| `manifest.json` | Dataset, configuraciones, conteos e inventario de integridad. |
| `SHA256SUMS` | Hashes de los otros seis archivos. |

Los identificadores vinculan cada juicio con un resultado y una respuesta.
`output_sha256` identifica los bytes UTF-8 de `raw_output`, sin reformatearlos.
`evidence` contiene las citas o la verificación que fundamenta cada juicio;
`location`, cuando está presente, indica si la evidencia textual está presente
o ausente en la respuesta.
`reviewed_fields` delimita los campos de prosa evaluados cuando corresponde.

Las fechas, modelos y configuraciones de cada respuesta identifican sus
condiciones de generación. La latencia describe el intento seleccionado y su
alcance se indica cuando está registrado; no representa la duración total de
la campaña. Los tokens se suman sólo para valores informados, con cobertura y
desconocidos explícitos. Un valor desconocido permanece `null`; no se interpreta
como cero. Estas métricas no establecen un importe facturado.
La implementación medida se identifica en `manifest.json.solution`; cada
respuesta conserva `solution_source_commit` cuando está atestiguado. Un commit
desconocido queda `null`. Los hashes de prompts y configuraciones se vinculan
desde cada respuesta.

## Integridad y límites

Desde esta carpeta:

```bash
sha256sum --check SHA256SUMS
```

Los archivos JSONL contienen un objeto JSON por línea. El manifiesto permite
verificar hashes, tamaños y conteos. El dataset está en
[`LoResuelvo_US60_evals_v1.0.0`](../../datasets/LoResuelvo_US60_evals_v1.0.0/).

Las evaluaciones deterministas y semánticas de estas 312 respuestas están
revisadas y aprobadas. Cada modelo respondió tres veces a cada uno de los mismos
52 casos. Las repeticiones permiten observar variaciones en las respuestas,
pero no agregan situaciones distintas a la muestra. Por eso, las métricas describen
lo observado en estos casos y no garantizan el mismo desempeño ante cualquier
otra consulta.
