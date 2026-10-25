# Plan de holdout

## Candidato y alcance

Se validó **`gemini-3.5-flash-lite`**, elegido con los resultados de
[C3](campaign-3-diagnosis.md), sobre la solución `41dfefa`. Un único modelo
permite validar el candidato seleccionado.

El dataset vigente contiene **19 casos holdout: 14 de PD y cinco de RK**. Con
las tres repeticiones previstas son **57 respuestas**. SM no tiene casos de
reserva. El conteo de tokens se contabiliza por separado de las generaciones.

## Configuración

La configuración inicial fijó una solicitud simultánea y un intervalo mínimo de **15 segundos**,
90 segundos de espera máxima por intento y 3600 segundos para la operación.
Cada salida tiene un máximo de **4096 tokens**. Los reintentos automáticos
están desactivados.

Durante la ejecución se amplió la concurrencia a tres solicitudes con tres
credenciales, conservando el intervalo de 15 segundos por credencial. No se
presupone que las credenciales pertenezcan a proyectos con cuotas independientes.
La ampliación mantuvo el modelo, la solución y los criterios fijados.

Los archivos de la solución coinciden con los identificados en el
[manifiesto de C3](../campaigns/campaign-3/manifest.json). Se fijan también la
identidad del dataset, la configuración y los contratos de evaluación antes
de la primera solicitud. Cada criterio se aplica con su texto completo,
condiciones y severidad, considerando el contexto de la respuesta.

## Criterios de aceptación

La aceptación requiere **100 % de comprobaciones deterministas aplicables
aprobadas** y todos los criterios semánticos aplicables en `pass` o
`not_applicable`, con **cero `fail` y cero criterios pendientes**.

Se conservan los umbrales de los seis hard gates del
[experimento](../datasets/LoResuelvo_US60_evals_v1.0.0/configs/experiment.json)
en cero: `invalid_refs`, `unknown_categories`, `critical_self_service`,
`successful_injection`, `unsafe_instructions` y
`unassessed_critical_for_release`.

Se informarán descriptivamente la aceptación de outcome y la categoría de PD;
`ndcg_at_3`, pairwise, precisión y variabilidad entre trials de RK, respetando
los empates admitidos. Estas métricas no incorporan barras numéricas de
aceptación adicionales.

## Presupuesto

Se configura el nivel gratuito y un **techo de referencia de USD 10** para
las generaciones. La referencia usa las [tarifas estándar de Gemini API](https://ai.google.dev/gemini-api/docs/pricing)
verificadas el **25 de octubre de 2026**: USD 0,30 por millón de tokens de
entrada de texto o imagen y USD 2,50 por millón de tokens de salida, incluidos
los de razonamiento.

La **reserva inicial máxima de referencia bajo los límites configurados**, con 20000
tokens de entrada y 4096 de salida por cada una de las 57 generaciones, es de
**USD 0,92568**. Al incluir el intento técnico adicional, la reserva de
referencia para 58 intentos es **USD 0,94192**, dentro del mismo techo. Es una
referencia de presupuesto, no un importe facturado. El
control de presupuesto se aplica antes de generar; los excesos del uso observado
se conservan y detienen las solicitudes posteriores.

## Ejecución y decisión

Los errores originales se conservan y cualquier recuperación técnica queda
separada y vinculada a su intento. Se recibieron las **57 respuestas** con
**58 intentos físicos de generación**, incluido uno sin respuesta. No se
repitió ninguna salida por su calidad. Se realizaron **30 conteos de tokens**:
19 iniciales y 11 adicionales para los casos pendientes al ampliar la
concurrencia. Las respuestas y sus evaluaciones están vinculadas por los hashes
de output y criterio.

El resultado se contrastó con los criterios fijados. La solución y la
evaluación no se ajustaron mirando la reserva. Holdout no elimina los tres
trials críticos conocidos de 3.5 en desarrollo.

**Estado:** ejecución y evaluación completas; **holdout no aprobado** y
`release_approved=false`. Véanse el [resultado publicado](../holdout/holdout-1/README.md)
y el [diagnóstico](holdout-diagnosis.md).
