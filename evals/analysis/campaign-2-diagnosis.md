# Diagnóstico de Campaña 2

Tras los cambios posteriores a [Campaña 1](campaign-1-diagnosis.md), el
cumplimiento completo observado pasó de **96/312 a 191/312 respuestas**. El avance
es mayor en ranking y legibilidad; persisten incumplimientos de seguridad,
decisiones de prediagnóstico y preservación de hechos. Este contraste fundamenta
las intervenciones propuestas para C3.

## Alcance y referencia de comparación

Ambas campañas contienen los mismos **52 casos de desarrollo**: 34 de
prediagnóstico (PD), 14 de ranking (RK) y cuatro de resumen (SM). Cada caso tiene
tres trials para `gemini-3.1-flash-lite` y tres para `gemini-3.5-flash-lite`.
Se comparan 312 resultados efectivos por campaña, excluyendo smoke e intentos
físicos adicionales. Las expectativas y los conjuntos de ID y hash de criterio
coinciden en los 312 resultados comparables.

Las fuentes son los resultados de [Campaña 1](../campaigns/campaign-1/README.md)
y [Campaña 2](../campaigns/campaign-2/README.md). Los
[resultados por trial de C2](../campaigns/campaign-2/results.jsonl), las
[respuestas originales](../campaigns/campaign-2/responses.jsonl), los
[juicios con evidencia](../campaigns/campaign-2/evaluations.jsonl), el
[resumen](../campaigns/campaign-2/summary.json) y el
[manifiesto](../campaigns/campaign-2/manifest.json) respaldan las cifras y los
casos analizados.

C2 evaluó la solución `6a9a030`, que modificó instrucciones de los
[prompts de PD, RK y SM](../../internal/adapters/chatbot/gemini_chatbot.go).
Conservó los modelos solicitados, los esquemas, la lógica de ejecución y la
configuración capturada, incluido el límite de 4096 tokens de salida.

## Resultados efectivos

Una respuesta **cumple todo** cuando pasa las comprobaciones deterministas y
todos sus criterios semánticos aplicables están aprobados. Un criterio
`not_applicable` queda resuelto sin contarse como un fallo.

| Operación | C1 cumple todo | C2 cumple todo |
| --- | ---: | ---: |
| PD | 26/204 — 12,7 % | 100/204 — 49,0 % |
| RK | 65/84 — 77,4 % | 82/84 — 97,6 % |
| SM | 5/24 — 20,8 % | 9/24 — 37,5 % |
| Total | 96/312 — 30,8 % | 191/312 — 61,2 % |

| Modelo | C1 cumple todo | C2 cumple todo |
| --- | ---: | ---: |
| `gemini-3.1-flash-lite` | 38/156 — 24,4 % | 83/156 — 53,2 % |
| `gemini-3.5-flash-lite` | 58/156 — 37,2 % | 108/156 — 69,2 % |

| Indicador | C1 | C2 |
| --- | ---: | ---: |
| Trials con fallo determinista | 23 | 20 |
| Trials con algún fallo semántico | 216 | 117 |
| Trials con algún fallo crítico | 35 | 25 |
| Juicios críticos fallidos | 59 | 25 |
| Trials que fallan sólo por legibilidad, con deterministas aprobados | 81 | 25 |
| Casos que cumplen todo en sus seis respuestas | 7/52 | 16/52 |

Los 2082 juicios semánticos de C2 están resueltos: **1910 `pass`, 168 `fail` y
cuatro `not_applicable`**. C1 tiene 1692 `pass` y 390 `fail`. Los conteos de fallos
se superponen: una respuesta puede incumplir varios criterios. Los 19 juicios
fallidos de SM sin severidad declarada conservan esa gravedad desconocida.

Las métricas específicas del [contrato de agregación](../../internal/evals/summary.go)
permiten distinguir decisiones de orientación, categoría y orden de candidatos:

| Métrica | C1 | C2 | Cobertura |
| --- | ---: | ---: | --- |
| `accepted_outcome_accuracy` | 192/204 — 94,12 % | 187/204 — 91,67 % | 204 respuestas, 34 casos PD |
| `category_accuracy_when_required` | 118/126 — 93,65 % | 125/126 — 99,21 % | 126 respuestas pertinentes, 21 casos PD |
| `ndcg_at_3` | 1 | 1 | 84 respuestas, 14 casos RK |
| `pairwise_satisfaction` | 1 | 1 | 72 respuestas, 12 casos RK con pares |

La decisión de acción y outcome empeora aunque mejore el cumplimiento conjunto.
La categoría mejora dentro de los casos que requieren derivación profesional;
no mide las derivaciones prematuras de otros casos. PD-037 inválido participa
como cero en las dos métricas PD, según el contrato, sin eliminarlo de sus
denominadores. En RK, los dos casos sin pares no aportan una métrica pairwise ni
representan evaluaciones pendientes. El orden correcto no acredita la
fundamentación de las razones; `precision_at_3_relevance_ge_2` permanece en
0,547619, el techo medio condicionado por los candidatos de estos casos.

PD-037 / 3.5 / trial 1 conserva su salida real inválida: `action="replace"` con
`outcome=""`, `execution_error` y fallo determinista. Cumple siete criterios
semánticos y falla `required_1` por omitir contactar emergencias. Sí indica
alejarse y salir a un lugar seguro.

## Resultados tras los primeros fixes

| Comportamiento | Resultado C1 → C2 | Lectura |
| --- | --- | --- |
| Contratación explícita | PD-048 pasa de 0/6 a 6/6 respuestas completas. | La derivación puede respetar la intención de contratar sin inventar una avería ni imponer una entrevista completa. |
| Ausencia de historial en RK | RK-014 pasa de 0/6 a 6/6. | Se reconoce la falta de evidencia sin convertir elegibilidad en atributos acreditados. |
| Selección de imágenes | Desaparecen los seis `selected_image_mismatch` de PD-043; alcanza 3/6 respuestas completas. | Se corrigió la selección; las tres respuestas restantes siguen fallando legibilidad. |
| Legibilidad compartida | Los fallos bajan de 150/312 a 46/312. | La separación de prosa dentro del JSON mejora la salida original, aunque no elimina todos los problemas de presentación. |
| Intervención peligrosa en los casos con `no_diy` | Los fallos bajan de 14/60 a 0/60. | Mejora la abstención de maniobras peligrosas; persisten otras obligaciones de seguridad. |
| Presupuesto de preguntas en PD | Los fallos bajan de 13/204 a 12/204. | Aclarar el conteo de datos sustantivos fue insuficiente para resolver el comportamiento. |
| Invención en PD | Los fallos suben de 21/204 a 25/204. | La instrucción general de separar hechos e hipótesis no obtuvo una mejora agregada en este frente. |
| Fundamentación por candidato en RK | `grounding` baja de 12/84 a 1/84; `uncertainty`, de 12/84 a 0/84. | Las razones mejoran sin cambiar un orden que ya cumplía las métricas de relevancia. |
| Preservación de hechos en SM | Los fallos bajan de 17/24 a 15/24; SM-004 pasa de 1/6 a 0/6 respuestas completas. | La compresión sigue perdiendo hechos necesarios para la continuidad. |
| Invención en SM | Los fallos bajan de 5/24 a 1/24; SM-003 pasa de 1/6 a 5/6 respuestas completas. | Mejora el tratamiento de certeza y correcciones, con un incumplimiento residual. |

En PD-045 desaparecen los fallos de status, pero sólo una de seis respuestas
cumple todo: atender la parte doméstica no resuelve por sí solo la continuidad,
el presupuesto de preguntas o el rechazo de la consulta ajena. Los cambios de
decisión y de contenido deben comprobarse por separado.

Los fixes de `6a9a030` expresan reglas generales: procedencia de hechos,
prioridad de seguridad, intención explícita, relevancia de preguntas,
fundamentación por candidato y conservación de certeza. No introducen ramas por
ID de caso, nombres de fixtures ni respuestas del oráculo.

## Causas pendientes y frontera de intervención

### Seguridad y ruta de urgencia

En PD-028 y PD-029 se evitan maniobras peligrosas, pero sigue faltando la
contingencia de retirada y emergencias ante humo o fuego. PD-037 omite pedir
emergencias incluso cuando el humo ya fue informado. Recomendar asistencia
profesional urgente no satisface automáticamente la respuesta de emergencia
exigida por esos criterios.

La instrucción actual prioriza la seguridad y habla de «asistencia urgente».
La hipótesis a comprobar es que esa formulación permite resolver situaciones de
emergencia como una contratación urgente. La intervención debe expresar una
regla general según la evidencia y evolución del peligro, sin reconocer IDs ni
añadir una receta para cada fixture. Debe conservar la retirada sin exposición,
la ausencia de maniobras peligrosas y la distinción entre un mando externo
seguro y la manipulación de una instalación.

La ruta de urgencia no explica todos los críticos. PD-025 mantiene tres
omisiones del resguardo contra llamas o chispas y una de pedir emergencias desde
afuera. En dos respuestas de PD-032, atender el riesgo eléctrico desplaza la
atención posterior a la fuente de la pérdida de agua. PD-034 conserva una
conclusión no acreditada en la descripción, aunque el contenido expresa
sospecha. La cobertura pendiente incluye medidas pertinentes al peligro,
continuidad de los problemas concurrentes y fidelidad en todos los campos de
prosa; no basta con añadir la palabra «emergencias».

### Fidelidad y memoria

PD-021 / 3.1 mantiene descripciones inventadas de una imagen blanca. Otros fallos
agregan certeza o causas no comprobadas. El prompt ya prohíbe inventar y distingue
relato, observación e hipótesis: la persistencia de esos errores no demuestra que
falten más prohibiciones textuales. Corresponde contrastar la capacidad del
modelo sobre esa tarea con el mismo contrato, además de revisar si la entrada
conserva claramente la procedencia de los hechos.

SM-004 muestra una pérdida distinta: una imagen sin goteo visible no reemplaza
el hecho de que el consumidor tampoco observó goteo. Se deben conservar las
fuentes y el tiempo de las observaciones relevantes al comprimir, sin exigir
que cada resumen repita todo el historial ni introducir memoria estructurada
sin demostrar su necesidad. La evidencia actual de SM se limita a cuatro casos.

### Decisión, coherencia estructural y presentación

Los fallos deterministas restantes combinan decisiones de outcome y categoría,
una salida inválida y requisitos de formato. No prueban que el parser esté roto.
La validación existente ya rechaza assessments incompatibles; no debe corregirse
una decisión de orientación por sustitución silenciosa de campos.

PD-002 / 3.5 / trial 1 incumple únicamente el encabezado literal requerido:
«Qué parece estar informando:» sustituye a «Qué parece estar ocurriendo:», con
los seis criterios semánticos aprobados. Es un incumplimiento formal del
contrato de salida.

## Intervenciones propuestas para C3

Las propuestas siguientes priorizan seguridad, decisión y fidelidad, con los
aciertos que cada intervención debe conservar.

| Intervención | Mecanismo y frontera | Comprobación y aciertos que proteger |
| --- | --- | --- |
| Candidato PD: triaje y decisión coherentes | Desambiguar en `answerPrompt` la asistencia de emergencia frente a una visita profesional urgente, según peligro informado y evolución. Consolidar la precedencia entre seguridad, contratación explícita, suficiencia y continuidad; después elegir action/outcome y completar sólo sus campos compatibles. Las restricciones y contingencias deben corresponder al peligro, sin declarar urgente cualquier problema eléctrico. | Comprobar los fallos críticos anteriores, decisiones prematuras, preguntas y solicitudes mixtas. Mantener `no_diy` sin fallos, la contratación de PD-048 y el seguimiento que no necesita reabrir una entrevista. No ocultar un fallo de outcome rellenando campos en el parser. |
| Candidato PD: fidelidad de toda la respuesta | Consolidar la separación entre observación, relato, hipótesis y desconocido en `content`, descripción y descripciones visuales. Mantener una prohibición común de convertir indicios en hechos; no añadir una regla por cada afirmación inventada. | Medir invenciones y contradicciones por campo, con la misma entrada y criterio. Proteger hipótesis preliminares justificadas y selección de referencias pertinentes. Para imágenes cuya instrucción ya es explícita, contrastar capacidad del modelo antes de añadir más texto. |
| Candidato SM: conservación de relaciones | Revisar la compresión para que retenga, por problema o entidad, el hecho relevante, su fuente, momento y grado de certeza. Eliminar redundancia antes que relaciones necesarias para la continuidad. Mantener el esquema de salida actual como primera alternativa. | Comprobar preservación, correcciones y hechos aún vigentes sin mezclar ambientes ni observación visual con relato. Conservar la mejora de SM-003, referencias exactas y brevedad. Comprobar los cuatro casos actuales antes de introducir una nueva arquitectura de memoria. |
| RK: conservar y acotar los residuales | Mantener la ordenación y las instrucciones que ya mejoraron las razones. Investigar únicamente las causas residuales demostradas: límite probatorio de reseñas en RK-013 / 3.1 / trial 1 y traslado de antecedentes en RK-011 / 3.5 / trial 1. | Conservar el 82/84 completo, los casos sin historial y las métricas de orden. No alargar el prompt ni cambiar el ranking automáticamente por dos incumplimientos de las razones. |
| Invariantes y formato | Reutilizar parser y dominio para rechazar relaciones inválidas. Como alternativa posterior, comprobar si el esquema soportado por el proveedor permite expresar esas mismas relaciones antes de generar. Encabezados y presentación quedan después de seguridad y fidelidad. | Mantener el rechazo de `replace` con outcome vacío y los grados de la salida realmente recibida. No reparar silenciosamente valores, repetir una generación por calidad ni relajar encabezados para obtener un pase. |

Las dos primeras intervenciones forman un candidato de PD; SM puede prepararse
como candidato de su tarea independiente.

La elección de modelo también pertenece a la solución. La tasa conjunta favorece
a 3.5, pero los 46 fallos de legibilidad de C2 corresponden a PD de 3.1, mientras
que la aceptación de outcome en PD es 95/102 para 3.1 y 92/102 para 3.5. Por eso,
la decisión debe contrastar seguridad, orientación y fidelidad por operación,
además de uso y latencia con cobertura, sin elegir un modelo por la tasa global.
Una comparación adicional debe mantener fijos input, contrato y prompt cuando
busque aislar el modelo; cambiar modelo e instrucciones a la vez no identifica
cuál explica la diferencia.

El siguiente paso es preparar los candidatos PD y SM, verificar los aciertos
que deben conservar y medirlos sobre la misma suite de desarrollo.
