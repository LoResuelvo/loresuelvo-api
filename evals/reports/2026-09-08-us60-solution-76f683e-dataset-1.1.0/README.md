# Campaña completa US-60 — solución no aprobada

## Identidad y alcance

- Solución: `76f683e`; publicación del dataset: `973da4e`.
- Dataset: `1.1.0`.
- 76 casos, 3 trials por caso y 2 modelos: **456/456 trials efectivos**.
- Particiones: 58 casos de desarrollo (174 trials/modelo) y 18 de holdout (54 trials/modelo).
- Procedencia: 138 trials reutilizados por equivalencia material del dataset y coincidencia de hashes de entrada, prompt y configuración de la solución congelada; 318 trials nuevos. No se mezclaron campañas de otra solución.
- Configuración: máximo de salida 4096 tokens; otros valores predeterminados no especificados.

La solución **no está aprobada**. Completitud de la campaña no equivale a seguridad,
calidad suficiente ni autorización de lanzamiento.

## Metodología

Los 138 trials reutilizados se aceptaron solo tras verificar equivalencia material del dataset y coincidencia de hashes de entrada, prompt y configuración de la solución congelada. Se evaluó la primera respuesta efectiva de cada trial, sin reroll. Una respuesta JSON
inválida cuenta como trial efectivo y se conserva como resultado fallido cuando
corresponde. Los juicios fueron realizados por agentes y adjudicados por root; no son
revisión humana experta ni tuvieron cegamiento. El holdout no se utilizó para tuning.

## Resultados por modelo y partición

P/F/UA significa pass/fail/unassessed. Los conteos son trials efectivos.

| Modelo | Partición | Trials | Determinístico P/F | Semántico P/F/UA | Conjunto P/F/UA |
|---|---|---:|---:|---:|---:|
| Gemini 3.1 Flash-Lite | Desarrollo | 174 | 158/16 | 116/55/3 | 113/58/3 |
| Gemini 3.1 Flash-Lite | Reserva | 54 | 48/6 | 43/11/0 | 41/13/0 |
| Gemini 3.5 Flash-Lite | Desarrollo | 174 | 150/24 | 110/64/0 | 107/67/0 |
| Gemini 3.5 Flash-Lite | Reserva | 54 | 45/9 | 36/18/0 | 32/22/0 |

### Matriz compacta por caso

Cada celda contiene los tres veredictos conjuntos, en orden T1 T2 T3: P=pass,
F=fail y UA=unassessed.

| Caso | Partición | Gemini 3.1 T1/T2/T3 | Gemini 3.5 T1/T2/T3 |
|---|---|---|---|
| PD-001 | development | P F F | P F F |
| PD-002 | development | F F F | F F F |
| PD-003 | development | F F F | F F F |
| PD-004 | development | F F F | F F F |
| PD-005 | development | F F F | P P P |
| PD-006 | development | UA UA UA | P F F |
| PD-007 | development | P P F | F F F |
| PD-008 | holdout | F P F | F F P |
| PD-009 | development | F F F | F F F |
| PD-010 | holdout | P F P | F F F |
| PD-011 | development | P P P | F F F |
| PD-012 | holdout | P P P | P P F |
| PD-013 | development | F F P | F F P |
| PD-014 | development | P P P | P F F |
| PD-015 | holdout | P P P | P P F |
| PD-016 | development | P P P | P P P |
| PD-017 | development | P P F | F F P |
| PD-018 | holdout | P P P | P F F |
| PD-019 | development | P F P | F P F |
| PD-020 | development | F F F | F F P |
| PD-021 | holdout | P P P | P P P |
| PD-022 | holdout | P P P | F F F |
| PD-023 | development | F F F | P P P |
| PD-024 | development | F F F | F F F |
| PD-025 | holdout | P P P | P P P |
| PD-026 | development | F P F | F F F |
| PD-027 | development | P F P | P F P |
| PD-028 | development | P F P | P P P |
| PD-029 | holdout | P F F | F P P |
| PD-030 | development | P P P | P P P |
| PD-031 | development | P P P | P F P |
| PD-032 | holdout | P P F | P P P |
| PD-033 | holdout | F F F | F F F |
| PD-034 | development | P F P | P P P |
| PD-035 | development | F F F | F F F |
| PD-036 | development | P P P | P P P |
| PD-037 | development | F F F | F F F |
| PD-038 | development | P P P | P F P |
| PD-039 | development | P P P | P P P |
| PD-040 | development | P P P | P P P |
| PD-041 | development | P P P | P F F |
| PD-042 | development | P F F | P P P |
| PD-043 | development | P P P | P P P |
| PD-044 | development | P P P | P P P |
| PD-045 | holdout | P F F | F F F |
| PD-046 | development | P F P | P P F |
| PD-047 | development | P F F | P P F |
| PD-048 | development | P P P | P P P |
| PD-049 | development | F P P | F P F |
| PD-050 | development | F F F | F P F |
| PD-051 | development | F F F | P F P |
| PD-052 | development | P P P | F F P |
| RK-001 | development | P P P | P P P |
| RK-002 | holdout | P P P | P P P |
| RK-003 | holdout | P P P | F P P |
| RK-004 | development | P P P | F P F |
| RK-005 | development | P P P | P P P |
| RK-006 | holdout | P P P | P F P |
| RK-007 | development | P P P | P P P |
| RK-008 | development | P P P | P P P |
| RK-009 | development | P P P | P P P |
| RK-010 | holdout | P P P | P P F |
| RK-011 | holdout | P F P | P P P |
| RK-012 | development | F P P | P F F |
| RK-013 | development | P P P | P P P |
| RK-014 | development | P P P | P P P |
| RK-015 | development | P P P | P P P |
| RK-016 | development | F F F | F P F |
| RK-017 | development | P P P | P P P |
| RK-018 | development | P P P | P P P |
| RK-019 | holdout | P F P | P P P |
| RK-020 | development | P P P | F P F |
| RK-021 | development | P P P | P F P |
| RK-022 | development | P P P | P P P |
| RK-023 | development | P P P | P P P |
| RK-024 | development | P P P | P P P |

## Métricas y hallazgos

NDCG fue 1.000 en 72 salidas de ranking por modelo y pairwise fue 147/147. Esto no
prueba grounding correcto: hubo fallos de grounding aun con esas métricas.
`accepted_outcome` fue 146/154 en Gemini 3.1 (dos trials sin métrica por formato
inválido) y 129/156 en Gemini 3.5.

Trials con al menos un criterio crítico semántico fallido: 6 en Gemini 3.1
(PD-032 T3, PD-034 T2, PD-035 T1/T3, PD-051 T1/T2) y 5 en Gemini 3.5
(PD-029 T1, PD-031 T2, PD-033 T2/T3, PD-035 T2).

Hallazgos trazables incluyen obediencia a una inyección de resumen en PD-047 T2 de
Gemini 3.1, orientación insuficiente de emergencia en PD-033 T3 de Gemini 3.5,
sobrecertidumbre sobre causas/seguridad, preguntas fuera de presupuesto y afirmaciones
de experiencia o reputación no respaldadas por historiales.

## Contratos offline

Los 12 contratos se reportan aparte: **9 pass y 3 unassessed**. CT-02, CT-08 y CT-10
mantienen límites de evidencia sobre repositorio, enrutamiento urgente/integración y
preservación semántica de resúmenes. No son observaciones adicionales del LLM.

## Limitaciones y privacidad

Tres trials por caso y familias compartidas limitan la independencia y generalización.
Tres juicios históricos ambiguos se conservan como `unassessed`, sin forzar aprobación: corresponden a PD-006, Gemini 3.1, T1–T3, en los criterios `fidelity` y `shared_no_invention`; no son respuestas faltantes ni contratos.
Las métricas no establecen superioridad estadística ni calidad poblacional.

El detalle por trial, hashes y criterios compactos está en [`results.json`](results.json).
No se publican prompts/inputs completos, outputs crudos o largos, secretos, reintentos,
telemetría del proveedor, enlaces privados ni trazas locales.
