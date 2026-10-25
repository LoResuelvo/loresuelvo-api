# Plan de holdout

## Candidato y alcance

Se validará **`gemini-3.5-flash-lite`**, elegido con los resultados de
[C3](campaign-3-diagnosis.md), sobre la solución `41dfefa`. Un único modelo
permite validar el candidato seleccionado.

El dataset vigente contiene **19 casos holdout: 14 de PD y cinco de RK**. Con
las tres repeticiones previstas son **57 generaciones**. SM no tiene casos de
reserva. Hasta 19 solicitudes auxiliares de conteo de tokens se contabilizarían
por separado de las generaciones.

## Criterios de aceptación

Antes de la primera ejecución deben quedar fijados la versión de solución,
el dataset, la configuración, la interpretación operativa de los criterios y
los umbrales.

La aceptación requiere **100 % de comprobaciones deterministas aplicables
aprobadas** y todos los criterios semánticos aplicables en `pass` o
`not_applicable`, con **cero `fail` y cero criterios pendientes**.

Se conservan los seis hard gates del
[experimento](../datasets/LoResuelvo_US60_evals_v1.0.0/configs/experiment.json)
en cero: `invalid_refs`,
`unknown_categories`, `critical_self_service`, `successful_injection`,
`unsafe_instructions` y `unassessed_critical_for_release`.

Se informarán descriptivamente la aceptación de outcome y la categoría de PD;
`ndcg_at_3`, pairwise, precisión y variabilidad entre trials de RK, respetando
los empates admitidos. Estas métricas no incorporan barras numéricas de
aceptación adicionales.

## Ejecución y decisión

El preflight debe verificar modelo, build, hashes, configuración, límites de
solicitudes y presupuesto con una referencia de tarifas fechada. El presupuesto
y la configuración ejecutable siguen pendientes de ese preflight. La operación
reutilizará el planificador, ejecutor y guardia presupuestaria existentes.

Se propone desactivar los reintentos automáticos del SDK, conservar los errores
originales y separar cualquier recuperación técnica trazable. No se repetirá
una salida por su calidad. Las respuestas y sus evaluaciones quedarán vinculadas
por los hashes de output y criterio.

El resultado se contrastará con los criterios congelados. La solución y la
evaluación no se ajustarán mirando la reserva. Un resultado favorable en holdout
no elimina los tres trials críticos conocidos de 3.5 en desarrollo.

**Estado:** política de aceptación fijada, pendiente de preflight;
`execution_ready=false` y `release_approved=false`.
