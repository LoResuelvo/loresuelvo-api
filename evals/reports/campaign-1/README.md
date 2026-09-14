# Campaña 1 — solución original

Primera medición del dataset golden `LoResuelvo_US60_evals_v1.0.0` sobre la solución original congelada. Fecha de ejecución y verificación de precios: **2026-09-14 UTC**.

## Alcance y trazabilidad

- **Smoke:** 18 slots por modelo, 36 totales.
- **Core primario:** 54 casos development × 3 trials × 2 modelos = 324 slots.
- **Total planificado:** 360 slots. El denominador proviene del protocolo completo, no de las filas ejecutadas.
- **Excluido:** holdout y campaña metamórfica; no hubo llamadas pagas sobre esos conjuntos.
- **Baseline original:** `7c2c88a43b2f3d7c2b1909b3bf5c50fb507eb169`.
- **Runner de recuperación:** `fe9ada39c03e9a8ac96ea272727aea0e926a7be6`.
- **Reporter:** `fcf475319728ae8c7e1d25240f4ddaa7e3427e70`.
- **Dataset manifest:** `063a87355225013229cb2415bbb96d99a41b0de97525f2a5e99fcf7df813802e`.
- **Protocolo:** `2086720983d425dd3c17f976b279090e7b29a9fe5b351b990695ca92decfe874`.
- **Addendum de recuperación:** `a9a8decb64034b29642876075355e154a5369da062db8c9320e647b3cdcba870`.

Los hashes completos de artefactos y revisiones están en `bundle-manifest.json` y `SHA256SUMS`.

## Estado de cobertura

| Indicador | Resultado |
|---|---:|
| Slots terminales | 360 / 360 |
| Respuesta física final por slot | 360 / 360 |
| Salida parseada/evaluable | 356 / 360 |
| Contenido recibido pero no parseable | 4 / 360 |
| Slots recuperados | 21 / 21 |
| Criterios semánticos evaluados por agente | 2031 / 2040 |
| Criterios no evaluables | 9 / 2040 |
| Revisión humana | 0 / 2040 |

`report.json.status.responses_complete=false` es correcto: ese campo significa cobertura de salida **parseada/evaluable**, no mera recepción física. Los cuatro casos restantes recibieron contenido, pero violaron el contrato de esquema; no se reintentaron porque la recuperación sólo admitía ausencia transitoria de respuesta.

La campaña original realizó 360 llamadas. La recuperación append-only realizó 32 llamadas adicionales: 21 primeras respuestas efectivas y 11 errores transitorios previos. Conserva cada intento, detiene el slot en su primera respuesta efectiva y no reemplaza los diarios originales.

## Resultados primarios

| Modelo | Prediagnóstico evaluable | Outcome accuracy | Category accuracy* | Casos variables outcome | Ranking evaluable | NDCG@3 | Precision@3 |
|---|---:|---:|---:|---:|---:|---:|---:|
| Gemini 3.1 Flash-Lite | 108 / 108 | 84.3% | 87.7% | 4 / 36 | 54 / 54 | 1.000 | 0.556 |
| Gemini 3.5 Flash-Lite | 108 / 108 | 84.3% | 86.0% | 12 / 36 | 51 / 54 | 1.000 | 0.556 |

\* Sólo cuando el caso exige categoría: 57 observaciones por modelo.

La variación entre trials fue mayor en prediagnóstico para 3.5: 12/36 casos variaron en outcome y 6/19 en categoría, frente a 4/36 y 1/19 para 3.1. En ranking no hubo variación de NDCG@3 ni Precision@3 entre los casos comparables. Las tres salidas de ranking no evaluables de 3.5 devolvieron una raíz JSON array en vez del objeto requerido.

Latencia de la campaña original: 3.1 tuvo p50 **2446 ms** y p95 **26886 ms**; 3.5 tuvo p50 **2620 ms** y p95 **44319 ms**. Estas cifras no mezclan la latencia de recuperación.

## Failure modes observados

1. **Afirmaciones no comprobadas:** los fallos semánticos más frecuentes fueron `fidelity` (65 criterios) y `shared_no_invention` (64). Se observaron diagnósticos, ubicaciones o causas expresados con más certeza que la evidencia disponible.
2. **Requisitos específicos omitidos o contradichos:** `required_1` falló 44 veces y `forbidden_1`, 21. Incluye omitir medidas de seguridad/no uso o sugerir intervenciones que el caso prohibía.
3. **Contrato de prediagnóstico:** 17 `outcome_mismatch`, 6 `category_mismatch`, 4 `action_mismatch`, 7 `selected_image_mismatch` y 7 fallos de encabezados/orden. Una misma respuesta puede aportar varios códigos.
4. **Esquema estructurado:** hubo 19 ocurrencias de `output_schema`; cuatro dejaron el slot sin salida parseable aun cuando sí hubo contenido físico.
5. **Presupuesto de preguntas:** `shared_question_budget` falló 19 veces, normalmente por pedir demasiada información o repetir datos ya aportados.
6. **Transitorios del proveedor:** la corrida original tuvo 21 slots sin respuesta (503 o timeout/deadline). Se recuperaron los 21; esto se reporta como incidente operacional, no como failure mode de calidad de la solución.

Los fallos semánticos por slot en primary fueron 52/162 para 3.1 y 60/162 para 3.5. La evaluación es de **agente**, ligada por hash a cada salida y criterio; no equivale a certificación humana.

## Baselines offline y límites

- Contratos offline: 9 pass, 3 unassessed.
- Mejor baseline determinístico de ranking (`frozen_lexical_heuristic`): NDCG@3 0.955, satisfacción pairwise 0.969 y Precision@3 0.556.
- El informe no aprueba release (`release_approved=false`) y deja explícita la revisión humana pendiente.
- `USD 7.210560` es el **upper bound conservador** original + recuperación.
- `USD 6.366080` es el **accounted conservador**, que incluye reservas ante usage metadata desconocida. Ninguna cifra representa costo observado en factura ni cargo facturado.
- `provider_usage_complete=false`; por eso `report.json` deja `cost=null` en las operaciones.

El bundle es transcript-free: no contiene prompts, imágenes, respuestas crudas, provider payloads, rutas privadas ni secretos. `report.json` conserva únicamente agregados y filas por caso/trial con hashes, estados, métricas y códigos necesarios para analizar failure modes.
