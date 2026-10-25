# Diagnóstico de Campaña 3

El cumplimiento completo observado pasa de **191/312 respuestas en C2 a
208/312 en C3**, de 61,2 % a 66,7 %. El avance se concentra en prediagnóstico;
ranking y resumen retroceden ligeramente. Los trials con algún fallo crítico
bajan de 25 a 11.

Las fuentes son los paquetes de [Campaña 2](../campaigns/campaign-2/README.md) y
[Campaña 3](../campaigns/campaign-3/README.md). Los
[resultados por trial](../campaigns/campaign-3/results.jsonl), las
[respuestas originales](../campaigns/campaign-3/responses.jsonl), las
[evaluaciones con evidencia](../campaigns/campaign-3/evaluations.jsonl) y el
[resumen](../campaigns/campaign-3/summary.json) contienen el detalle completo.

## Solución y alcance

C3 evalúa `41dfefa` sobre los mismos 52 casos de desarrollo de C2, con los mismos
dos modelos y tres trials por caso y modelo. Las expectativas y los conjuntos
de ID y hash de criterio coinciden en los 312 resultados comparados. La
definición de cumplimiento completo y el contrato de agregación se mantienen
como en el [diagnóstico de C2](campaign-2-diagnosis.md).

La solución consolida reglas de triaje, continuidad y fidelidad en
[PD y SM](../../internal/adapters/chatbot/gemini_chatbot.go): distingue
emergencias de visitas profesionales urgentes; preserva los problemas
concurrentes después de atender el peligro; usa los cambios materiales del
contexto para decidir si conserva o reemplaza la evaluación; y retiene fuente,
momento y certeza al resumir. Las instrucciones de RK se conservan. Son reglas
generales de comportamiento, sin ramas por ID de caso ni respuestas prefijadas
para los fixtures.

## Contraste con C2

| Operación | C2 cumple todo | C3 cumple todo |
| --- | ---: | ---: |
| PD | 100/204 — 49,0 % | 121/204 — 59,3 % |
| RK | 82/84 — 97,6 % | 79/84 — 94,0 % |
| SM | 9/24 — 37,5 % | 8/24 — 33,3 % |
| Total | 191/312 — 61,2 % | 208/312 — 66,7 % |

| Modelo | C2 cumple todo | C3 cumple todo | Trials críticos en C3 |
| --- | ---: | ---: | ---: |
| `gemini-3.1-flash-lite` | 83/156 — 53,2 % | 90/156 — 57,7 % | 8 |
| `gemini-3.5-flash-lite` | 108/156 — 69,2 % | 118/156 — 75,6 % | 3 |

| Indicador | C2 | C3 |
| --- | ---: | ---: |
| Trials con fallo determinista | 20 | 7 |
| Trials con algún fallo semántico | 117 | 104 |
| Trials con algún fallo crítico | 25 | 11 |
| Criterios críticos fallidos | 25 | 12 |
| Trials que fallan sólo por legibilidad, con deterministas aprobados | 25 | 15 |
| Casos que cumplen todo en sus seis respuestas | 16/52 | 16/52 |

Los 25 juicios fallidos de SM conservan su severidad no declarada. Los conteos
de críticos anteriores incluyen únicamente la severidad explícita del criterio.

La aceptación de action y outcome mejora de 187/204 a **198/204**. La categoría
en los casos que requieren derivación pasa de 125/126 a **126/126**. En ranking,
`ndcg_at_3` y `pairwise_satisfaction` permanecen en 1, con la misma cobertura;
el orden correcto sigue coexistiendo con fallos en la fundamentación.

## Comportamientos observados y fallos residuales

| Comportamiento | C2 → C3 | Resultado |
| --- | --- | --- |
| Precauciones y rutas de emergencia | 25 → 11 trials críticos | Mejora la cobertura, con incumplimientos aún presentes en ambos modelos. |
| Legibilidad en PD | 46 → 24 fallos | Los 24 fallos de C3 corresponden a 3.1. |
| Invención en PD | 25 → 22 fallos | Se reducen las adiciones sin respaldo, pero persisten en distintos campos de prosa. |
| Presupuesto de preguntas en PD | 12 → 19 fallos | La precedencia de decisiones mejora sin resolver el exceso de datos solicitados. |
| Fundamentación en RK | 1 → 3 fallos de `grounding` | El orden mantiene sus métricas y la fidelidad de algunas razones empeora. |
| Preservación en SM | 15 → 16 fallos | La conservación de relaciones sigue siendo insuficiente. |
| Invención en SM | 1 → 4 fallos | Persisten descartes o asociaciones que la entrada no acredita. |

Los fallos críticos restantes están en PD. En 3.1 incluyen condicionar la ruta
de emergencia a que persista o aumente un humo ya informado, omitir la atención
posterior a una pérdida de agua y convertir indicios en confirmación. En 3.5
quedan tres: una precaución contra llamas omitida ante olor intenso a gas,
ausencia de la contingencia de retirada y emergencias ante humo o fuego, y
atención eléctrica urgente subordinada a no poder cortar desde el tablero.
Los resultados y las razones de cada criterio delimitan esos incumplimientos.

En RK, respetar elegibilidad y relevancia no basta para acreditar los hechos
de cada candidato. En SM, la mejora de instrucciones no se traduce en mayor
cumplimiento conjunto: SM-003 pasa de cinco a dos respuestas completas y la
preservación de hechos sigue siendo el principal frente pendiente. Los cuatro
casos de resumen conservan su detalle en el paquete.

## Candidato para holdout

Se optará por `gemini-3.5-flash-lite` con la versión de solución evaluada en C3.
Obtiene más respuestas completas en las tres tareas: **72/102 en PD, 40/42 en
RK y 6/12 en SM**, frente a 49/102, 39/42 y 2/12 de 3.1, y acumula menos trials
críticos. En aceptación de action y outcome, 3.1 conserva una ventaja de
101/102 frente a 97/102; la elección prioriza el cumplimiento conjunto y la
seguridad, además de esa métrica de decisión.

C3 cierra la etapa de desarrollo con `release_approved=false`.
La reserva vigente contiene 14 casos de PD y cinco de RK; no incluye
SM. La decisión final debe conservar ese alcance de validación. La
[propuesta de holdout](holdout-plan.md) define la siguiente medición.
