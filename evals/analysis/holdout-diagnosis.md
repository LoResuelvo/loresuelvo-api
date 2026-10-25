# Diagnóstico de holdout

El candidato `gemini-3.5-flash-lite`, con la solución `41dfefa`, **no aprueba
holdout**: cumplen todas las comprobaciones **41 de 57 respuestas (71,9 %)**.
Se aplicó la condición fijada de que todos los criterios aplicables deben
aprobar. La solución, el modelo y los criterios permanecieron sin cambios.

Los [resultados de holdout](../holdout/holdout-1/README.md) contienen las
respuestas, los criterios y sus evidencias; el [plan](holdout-plan.md) conserva
el alcance, la configuración y las condiciones de aprobación.

## Resultado y contraste

| Tarea | C3, modelo 3.5 | Holdout, modelo 3.5 |
| --- | --- | --- |
| Prediagnóstico | 72/102 (70,6 %) | 26/42 (61,9 %) |
| Ranking | 40/42 (95,2 %) | 15/15 (100 %) |
| PD + RK | 112/144 (77,8 %) | 41/57 (71,9 %) |

La diferencia observada en PD + RK es de **−5,8 puntos porcentuales** entre
conjuntos de casos distintos. SM queda fuera del contraste porque no tiene
casos de reserva. El [diagnóstico de C3](campaign-3-diagnosis.md) desarrolla
los cambios respecto de la campaña anterior.

Los 366 criterios semánticos están completos: **350 `pass` y 16 `fail`**,
sin pendientes. Las comprobaciones deterministas aprueban en **52 de 57**
respuestas. Hay **seis fallos críticos**, en seis respuestas de dos casos.

## Comportamientos que fallan

| Comportamiento | Casos y trials | Resultado observado |
| --- | --- | --- |
| Protección ante riesgos | PD-027, t1–t3; PD-031, t1–t3 | Se omite impedir el reingreso sin autorización o mantener alejadas a otras personas, respectivamente. Son los seis fallos críticos. |
| Fidelidad a los datos | PD-016 t2 | Se agrega una extensión del corte que el consumidor no suministró. |
| Preguntas | PD-019 t1–t2; PD-023 t1 y t3 | Se excede el presupuesto de información solicitada; en PD-023 también se presupone un síntoma que el usuario no informó. |
| Límites de mantenimiento | PD-046 t2; PD-047 t1 | Se amplía la limpieza a suciedad adherida, o se fija un secado de 24 horas sin sustento y falta una condición de detención ante la persistencia del problema. |
| Clasificación prematura | PD-016 t1–t2 | Ante «se me cortó la luz», se deriva a Electricidad sin recabar la información que exige el caso. Falla el outcome y la categoría. |
| Categoría de autoservicio | PD-046 t1–t3 | La expectativa particular de PD-046 exige una categoría vacía; las tres respuestas incluyen una categoría. |

Las dos últimas filas describen comprobaciones deterministas contra las
expectativas de cada caso. La expectativa de PD-046 no implica que asignar
categoría sea incompatible con autoservicio en general. Sus resultados se
conservan y se distinguen de los criterios semánticos: son comprobaciones
diferentes sobre la misma respuesta.

## Decisión

**`release_approved=false`.** Holdout detectó omisiones críticas y no alcanzó
la condición de aprobación. Tampoco elimina los tres trials críticos conocidos
del modelo 3.5 en desarrollo. La revisión final de los 19 casos y sus
57 respuestas está completa. Estos resultados cierran la medición de reserva;
no se modificaron la solución, los criterios ni las condiciones de aprobación,
ni se repitieron respuestas para mejorar su calidad.
