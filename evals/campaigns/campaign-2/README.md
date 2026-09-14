# Campaña campaign-2 — evidencia longitudinal

Este directorio conserva los datos detallados necesarios para comparar la solución entre campañas. No contiene gráficas: las visualizaciones y métricas futuras deben derivarse de estos archivos.

## Alcance de la ejecución

- **Comprobaciones smoke:** 18 casos × 2 modelos = 36 slots (1 ejecución por caso y modelo).
- **Fase primaria:** 54 casos × 3 trials × 2 modelos = 324 slots.
Los smoke reutilizan casos del conjunto development: comprueban el circuito operativo antes de la fase primaria, pero no son casos nuevos ni observaciones independientes. Los trials también repiten los mismos casos para observar variabilidad; no aumentan la diversidad del dataset.

## Archivos

- **responses.jsonl:** una respuesta efectiva por slot, con texto original en `raw_output` y salida interpretada en `parsed_output` cuando fue posible. Las respuestas malformadas se conservan sin corregir. Para inspeccionar un caso, buscá su `dataset_case_id`, `requested_model` y `trial`; cada línea es una respuesta completa.
- **attempts.jsonl:** cada solicitud real al proveedor, incluidos los errores transitorios y la recuperación append-only; no contiene prompts, imágenes, headers ni identificadores HTTP.
- **results.jsonl:** resultado por caso, modelo y trial, con esperado, observado, métricas y códigos de fallo.
- **reviews.jsonl:** juicios por criterio, su evidencia y procedencia; effective_for_result distingue los juicios usados en el resultado de los reemplazados por recuperación.
- **summary.json:** agregados derivados y limitaciones de la campaña.
- **manifest.json:** identidad del golden, protocolo, commits, hashes de fuentes, cobertura y datos ausentes.
- **SHA256SUMS:** integridad de todos los artefactos anteriores.

## Cobertura

- Slots planificados: **360**.
- Solicitudes reales al proveedor: **404**.
- Respuestas físicas conservadas: **360**.
- Respuestas malformadas conservadas: **1**.
- Revisiones efectivas: **2035**; evaluadas por agente: **2035**; evaluadas por humanos: **0**; no evaluadas: **0**.

Las revisiones de esta campaña fueron realizadas por agentes salvo que `reviewer_kind` indique explícitamente `human`; la identidad de revisores es declarada, no autenticada. Una revisión de agente no certifica seguridad. Esta campaña no aprueba un release.
