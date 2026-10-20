# US75: lectura de detalle de conversaciones

Seguimiento: [API #235](https://github.com/LoResuelvo/loresuelvo-api/issues/235).
Implementación: `f693ebae5083869cbf630dd781d059087783f5dd`.
Baseline: `786f9446be743793d0b015ec01b3573877a2b640`.

`GET /conversations/{conversationId}` clasifica la conversación mediante una
proyección de acceso (`ID`, `Type`, `ConsumerID`, `ProviderID`). En conversaciones
de trabajo comprueba identidad y rol antes de ejecutar el lector de detalle
existente. Esto evita hidratar dos veces mensajes y adjuntos. El lector final
sigue obteniendo cabecera, estado, fecha de actualización y mensajes; conserva
los resolutores de foto y archivos. Chatbot conserva su lectura de agregado y
su tratamiento de assessment y recomendaciones, con una consulta adicional
para clasificar el acceso.

## Contrato HTTP

La revisión de handler, mapper y DTO contra
[`conversation-by-id.yaml`](../../openapi/paths/conversation-by-id.yaml),
[`ConversationDetail`](../../openapi/components/schemas/conversation-detail.yaml),
[`ConversationMessage`](../../openapi/components/schemas/conversation-message.yaml)
y [`ChatbotConversationDetail`](../../openapi/components/schemas/chatbot-conversation-detail.yaml)
no requiere modificar OpenAPI ni regenerar el bundle: la optimización no altera
el contrato público.

- Autenticación Bearer y acceso sólo al participante con el rol correspondiente:
  coincidir sólo con el ID de la contraparte no otorga acceso.
- `200` conserva `id`, `type`, `status`, `messages`, `updated_on` y el bloque
  `work` o `chatbot`. Se mantienen el orden cronológico, los metadatos y las URLs
  de imagen, audio y video, así como foto de contraparte y recomendaciones.
- Se conservan `400` para identificador inválido, `401` por autenticación,
  `403` por acceso denegado, `404` para conversación inexistente y `500` ante
  fallos inesperados, con las respuestas de error existentes.
- Un envío confirmado antes de la lectura final sigue visible. No se añade
  una transacción ni una garantía de snapshot o atomicidad adicional.

No hay nuevos endpoints, migraciones, índices, caché, cambios del pool ni
modificaciones del flujo de comandos o POST de chatbot.

## Validación funcional

La entrega cuenta con `make test` aprobado (926 escenarios y 9.114 steps),
`make lint` aprobado y cinco repeticiones aprobadas con detector de carreras
de la prueba dirigida de envío concurrente. La suite se ejecutó con Go 1.26.8;
el host de medición local utilizó Go 1.26.1.
La [ejecución CI 39357099279](https://github.com/LoResuelvo/loresuelvo-api/actions/runs/39357099279)
del commit de implementación finalizó correctamente.

La cobertura incorporada comprueba acceso work/chatbot, inexistencia y fallos
del lector; rechazo por identidad o rol antes del detalle; conversión de errores
de búsqueda de usuario en acceso denegado (`403`); propagación de fallos de
proyección, detalle y archivos; hidratación de metadatos persistidos de
audio/video; y un envío confirmado entre la proyección de acceso y la lectura
final, además de su visibilidad en un GET posterior.

En staging aprobaron los cinco controles de acceso: consumidor y prestador
ajenos a una conversación work (`403` en ambos casos), prestador frente a
chatbot (`403`), conversación inexistente (`404`) y petición sin autenticación
(`401`). Los cinco resultados se correlacionaron con sus logs API.

## Comparación local

Tres corridas A/B, alternando baseline/candidato, candidato/baseline y
baseline/candidato; seis casos por concurrencia (1 y 10), 150 peticiones medidas
y 20 de calentamiento por celda. Total: 72 celdas, 10.800 medidas y 1.440 de
calentamiento. Diez casos de equivalencia por versión y corrida suman otras
60 peticiones: mismo status, tamaño y hash del cuerpo, incluidos work de ambos
roles, adjuntos, acceso denegado, inexistencia y ambos estados de chatbot.

HTTP TCP loopback, PostgreSQL 16.13 aislado y fixture sintético de aproximadamente
50 MB: 100 conversaciones work con 99.010 mensajes, historias de 10 y 1.000
mensajes, adjuntos de imagen/audio/video y chatbot mínimo y con assessment y
recomendación. Servicio de archivos y metadatos persistidos reales, con
almacenamiento de URLs deterministas; no mide el proveedor externo de archivos.
La identidad persistida, el rol y la política de participante son reales;
la validación externa JWT/Auth0 queda fuera del harness local.

Pool: 20 conexiones máximas, 5 ociosas, vida máxima 30 minutos y tiempo ocioso
máximo 5 minutos. Logging WARN. Host compartido de 8 CPU, sin cuotas del
contenedor PostgreSQL. Se registraron SELECT, filas, aperturas de conexiones,
esperas del pool mediante `DB.Stats`, errores, mediana y p95; el tiempo observado
incluye HTTP y aplicación, y no representa ejecución SQL pura.

| Caso | SELECT baseline → candidato por GET | Filas baseline → candidato |
|---|---:|---:|
| Consumidor, 10 mensajes y adjuntos | 13 → 9 | 31 → 18 |
| Prestador, 10 mensajes y adjuntos | 14 → 10 | 32 → 19 |
| Consumidor, 1.000 mensajes | 10 → 6 | 2.002 → 1.002 |
| Prestador, 1.000 mensajes | 11 → 7 | 2.003 → 1.003 |
| Chatbot mínimo | 10 → 11 | 4 → 5 |
| Chatbot con assessment y recomendación | 14 → 15 | 7 → 8 |

Las cuatro consultas menos en work se repiten en todas las celdas. Los
conteos de historias cortas incluyen las consultas del servicio real de
archivos; por eso superan los de la exploración original con dobles.
No hubo errores ni incrementos de espera del pool. Las aperturas variaron
entre celdas y no permiten atribuir toda diferencia temporal al cambio.

Con concurrencia 1, mediana y p95 de los cuatro casos work mejoraron en las
tres corridas. Con concurrencia 10 la mejora temporal no es universal:

| Ejemplo, ms | Corrida | Baseline → candidato |
|---|---:|---:|
| Mediana consumidor, 10 mensajes | 1 / 2 / 3 | 6,10 → 13,96 / 12,19 → 12,67 / 12,93 → 9,71 |
| p95 prestador, 10 mensajes | 1 | 28,15 → 51,82 |
| p95 consumidor, 1.000 mensajes | 3 | 73,09 → 79,34 |
| p95 prestador, 1.000 mensajes | 3 | 58,48 → 66,32 |

Chatbot añade exactamente una consulta en ambos estados y muestra resultados
temporales mixtos. El ahorro estructural de work está demostrado; estos
ensayos cortos en un host compartido no demuestran una mejora universal de
latencia, ni una mejora porcentual de la interfaz.

## Comparación en staging

Baseline `v0.0.77`, correspondiente a `786f9446be743793d0b015ec01b3573877a2b640`,
comparado con `v0.0.78`, implementación
`f693ebae5083869cbf630dd781d059087783f5dd`, el 20 de octubre de 2026.
La [release 39357825904](https://github.com/LoResuelvo/loresuelvo-api/actions/runs/39357825904)
desplegó el candidato en staging
([deployment 7189883308](https://api.github.com/repos/LoResuelvo/loresuelvo-api/deployments/7189883308)).
PostgreSQL externo 17.6, arquitectura aarch64.
Se utilizaron cuentas y datos de prueba: seis conversaciones y
cuatro contrapartes auxiliares, respetando la unicidad del par de participantes.

Cada versión se midió en tres corridas de seis casos, concurrencia 1,
10 peticiones medidas y 5 de calentamiento por celda, con pausa de 100 ms
entre peticiones. Por versión: 18 celdas, 180 peticiones medidas y 90 de
calentamiento; las 270 devolvieron HTTP 200, sin errores, y sus request IDs
se correlacionaron con 270 logs API. La imagen y la instancia se mantuvieron
estables durante cada campaña, sin aumento del contador de reinicios.
El fixture permaneció igual entre versiones. Las respuestas conservaron
exactamente status, tamaño en bytes y hash del cuerpo sin normalización.

Mediana de las tres medianas por caso; tiempos en milisegundos. La variación
porcentual corresponde al cliente y se calcula con los valores sin redondear.

| Caso | Cliente baseline → candidato | Cambio cliente | API baseline → candidato |
|---|---:|---:|---:|
| Consumidor, 10 mensajes | 1.099,63 → 782,01 | −28,88 % | 801,00 → 487,50 |
| Prestador, 10 mensajes | 1.179,08 → 849,81 | −27,93 % | 882,00 → 569,00 |
| Consumidor, 1.000 mensajes | 1.803,72 → 1.308,08 | −27,48 % | 1.045,00 → 651,50 |
| Prestador, 1.000 mensajes | 1.905,95 → 1.361,01 | −28,59 % | 1.125,00 → 733,00 |
| Chatbot mínimo | 1.092,96 → 1.186,76 | +8,58 % | 801,00 → 892,50 |
| Chatbot con assessment y recomendación | 1.419,26 → 1.511,08 | +6,47 % | 1.121,50 → 1.217,50 |

Los cuatro casos work mejoraron en las tres corridas. La siguiente comparación
usa las medianas cliente: la separación es la menor mediana del baseline
menos la mayor del candidato; el rango es máximo menos mínimo dentro de las
tres corridas de cada versión.

| Caso work | Rango baseline, ms | Rango candidato, ms | Separación, ms |
|---|---:|---:|---:|
| Consumidor, 10 mensajes | 8,60 | 21,09 | 291,93 |
| Prestador, 10 mensajes | 0,53 | 31,95 | 299,59 |
| Consumidor, 1.000 mensajes | 73,07 | 13,90 | 412,40 |
| Prestador, 1.000 mensajes | 3,50 | 11,04 | 532,03 |

En estos casos, la reducción repetida supera la variación observada entre
corridas de ambas versiones. Chatbot empeoró en las tres corridas: el coste
cliente agregado fue +93,81 ms (+8,58 %) para el mínimo y +91,81 ms (+6,47 %)
con assessment y recomendación. El coste observado acompaña el compromiso
de una consulta adicional y se registra por separado del beneficio de work.

El tiempo cliente incluye la ruta pública y la transferencia del cuerpo; el
tiempo servidor procede del log HTTP. No se interpreta su diferencia como una
medición aislada de red. Con n=10, el p95 por celda coincide con el máximo
observado y es sólo descriptivo.
Las campañas baseline y candidato fueron sucesivas, sin alternar releases;
el tamaño de muestra y las condiciones del entorno limitan la extrapolación.
La medición verifica el beneficio de work en este escenario y registra el
coste chatbot; no demuestra mejora universal ni atribuye causalidad a la
latencia de la interfaz. Los conteos SQL y filas corresponden al ensayo local:
los logs de staging no incluyen trazas SQL de nivel DEBUG.

Tras la validación se retiraron las seis conversaciones de prueba, sus mensajes
y los cuatro usuarios auxiliares. Las dos cuentas de prueba preexistentes,
las tres conversaciones anteriores y sus seis mensajes se conservaron sin
cambios.

## Reversión

Ante regresión de autorización, contenido, errores o coste reproducible
inaceptable, revertir la implementación y desplegar la release correctiva por
el pipeline habitual. No hay migraciones ni estado nuevo que revertir.
