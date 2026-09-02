Feature: 13.3 Recomendar prestadores elegibles mediante IA según confianza, reputación y experiencia
    Como consumidor
    quiero que la IA priorice los prestadores elegibles según evidencia relevante y confiable
    para recibir recomendaciones fundamentadas y consistentes con mi problema

    Background:
        Given que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        And que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que el domicilio del consumidor "ana@example.com" pertenece a la zona de cobertura "Comuna 6"
        And que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA está disponible
        And que la IA de recomendación de prestadores está disponible

    Rule: La IA ordena solamente candidatos elegibles, fundamenta la recomendación con evidencia y respeta el máximo de 3 prestadores recomendados

        Scenario: 13.3.1-GP Recomendar en el orden elegido por la IA según la evidencia disponible
            Given existen los siguientes prestadores elegibles de "Plomería" en la zona "Comuna 6":
                | correo                      | nombre  | apellido |
                | juan.plomero@example.com    | Juan    | Gómez    |
                | pedro.plomero@example.com   | Pedro   | Dib      |
                | marcela.plomera@example.com | Marcela | Ruiz     |
                | roberto.plomero@example.com | Roberto | Paz      |
            And que esos prestadores tienen diferentes ratings, reseñas de consumidores y experiencia en trabajos pagados
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            And que la IA recomendará, en este orden, a "Marcela Ruiz", "Juan Gómez" y "Pedro Dib" con razones fundamentadas en la evidencia
            When envío un mensaje al chatbot asistido por IA:
                """
                Pierde agua la conexión bajo la pileta de la cocina.
                """
            Then el sistema muestra a "Marcela Ruiz", "Juan Gómez" y "Pedro Dib" en ese orden
            And la cantidad de prestadores mostrados no supera el máximo de 3 prestadores recomendados
            And cada prestador recomendado incluye las razones seleccionadas por la IA
            And persiste el ranking vigente con los candidatos considerados, la selección ordenada y sus razones

        Scenario: 13.3.2-GP Enviar a la IA todos y únicamente los prestadores elegibles por rubro y zona
            Given existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y zona de cobertura "Comuna 6"
            And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 14"
            And existe un prestador registrado con correo "laura.electricista@example.com", nombre "Laura", apellido "Suárez", rubro "Electricidad" y zona de cobertura "Comuna 6"
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            When envío un mensaje al chatbot asistido por IA:
                """
                Necesito reparar una pérdida de agua debajo de la pileta.
                """
            Then la IA de recomendación recibe como único candidato al prestador "Juan Gómez"
            And la IA de recomendación no recibe a "Pedro Dib" ni a "Laura Suárez"

        Scenario: 13.3.3-GP Construir evidencia estructurada distinguiendo su procedencia y confiabilidad
            Given existe un prestador elegible de "Plomería" llamado "Juan Gómez"
            And que "Juan Gómez" tiene trabajos pagados con ratings y reseñas escritas por consumidores
            And que "Juan Gómez" tiene informes de finalización escritos por él para trabajos pagados
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            When envío un mensaje al chatbot asistido por IA:
                """
                Tengo una pérdida recurrente en el sifón de la cocina.
                """
            Then la evidencia de "Juan Gómez" enviada a la IA incluye el promedio, cantidad y distribución de ratings
            And incluye la cantidad y recencia de sus trabajos pagados
            And incluye sus reseñas identificadas como opiniones escritas por consumidores
            And incluye sus informes de finalización identificados como evidencia autoescrita por el prestador

        Scenario: 13.3.4-GP Mantener como candidato a un prestador nuevo sin historial
            Given existe un prestador elegible de "Plomería" llamado "Juan Gómez" sin trabajos pagados, ratings, reseñas ni informes de finalización
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            When envío un mensaje al chatbot asistido por IA:
                """
                Se rompió una conexión de agua bajo la mesada.
                """
            Then la IA de recomendación recibe a "Juan Gómez" como candidato
            And su evidencia histórica se representa como vacía

    Rule: La evidencia enviada a la IA protege datos personales y contenido privado

        Scenario: 13.3.5-GP Anonimizar candidatos y excluir información privada
            Given existe un prestador elegible de "Plomería" llamado "Juan Gómez" con reseñas e informes de finalización
            And que sus informes de finalización incluyen imágenes privadas
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            When envío un mensaje al chatbot asistido por IA:
                """
                Necesito reparar una pérdida debajo de la pileta.
                """
            Then la IA de recomendación identifica al candidato mediante una referencia opaca
            And la IA de recomendación no recibe datos personales, fotos ni información sensible

    Rule: El ranking vigente se asocia a la evaluación actual y se reutiliza sin recalcularlo

        Scenario: 13.3.6-GP Consultar el ranking persistido sin volver a invocar a la IA
            Given que una evaluación vigente requiere un profesional de "Plomería"
            And que sus recomendaciones persistidas son "Marcela Ruiz", "Juan Gómez" y "Pedro Dib" en ese orden con sus razones
            When consulto el detalle de la conversación
            Then el detalle muestra "Marcela Ruiz", "Juan Gómez" y "Pedro Dib" en el orden persistido con sus razones
            And la IA de recomendación no vuelve a ser invocada

        Scenario: 13.3.7-GP Reutilizar el ranking cuando la evaluación vigente no cambia
            Given que una evaluación vigente requiere un profesional de "Plomería"
            And que sus recomendaciones persistidas son "Marcela Ruiz" y "Juan Gómez" en ese orden con sus razones
            And que el chatbot responderá sin modificar la evaluación vigente
            When continúo la conversación con información que no modifica el diagnóstico
            Then la respuesta conserva a "Marcela Ruiz" y "Juan Gómez" en el orden persistido con sus razones
            And la IA de recomendación no vuelve a ser invocada

        Scenario: 13.3.8-GP Reemplazar el ranking vigente cuando cambia la evaluación profesional
            Given que una evaluación anterior requiere un profesional de "Plomería"
            And que el ranking vigente tiene a "Juan Gómez" como recomendación
            And que el chatbot generará una nueva evaluación que requiere un profesional de "Plomería"
            And que la IA recomendará a "Marcela Ruiz" para la nueva evaluación
            When continúo la conversación con nueva información sobre el problema
            Then el sistema reemplaza el ranking vigente por uno con "Marcela Ruiz" asociado a la nueva evaluación
            And una consulta posterior devuelve a "Marcela Ruiz" sin reutilizar el ranking anterior

        Scenario: 13.3.9-GP Persistir una recomendación vacía cuando no hay candidatos elegibles
            Given que ningún prestador de "Plomería" cubre la zona "Comuna 6"
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            When envío un mensaje al chatbot asistido por IA:
                """
                Pierde agua la conexión debajo de la pileta.
                """
            Then el sistema persiste una lista vacía de prestadores recomendados para la evaluación
            And la IA de recomendación no es invocada
            And una consulta posterior devuelve la misma lista vacía

    Rule: Una recomendación inválida o indisponible no produce persistencia parcial

        Scenario Outline: 13.3.10-GP Rechazar referencias inválidas devueltas por la IA
            Given que "Juan Gómez" y "Marcela Ruiz" son los únicos candidatos elegibles
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            And que la IA devolverá una respuesta con <referencia inválida>
            When envío un mensaje al chatbot asistido por IA:
                """
                Necesito reparar una pérdida debajo de la pileta.
                """
            Then el sistema rechaza la recomendación inválida sin aplicar un orden alternativo
            And no persiste parcialmente el nuevo mensaje, la respuesta, la evaluación ni sus recomendaciones

            Examples:
                | referencia inválida             |
                | una referencia desconocida      |
                | una referencia duplicada        |
                | una referencia no elegible      |

        Scenario: 13.3.11-GP Evitar persistencia parcial cuando la IA de recomendación no está disponible
            Given existe un prestador elegible de "Plomería" llamado "Juan Gómez"
            And que el chatbot concluirá que se requiere un profesional del rubro "Plomería"
            And que la IA de recomendación de prestadores no está disponible
            When envío un mensaje al chatbot asistido por IA:
                """
                Necesito reparar una pérdida debajo de la pileta.
                """
            Then el sistema informa que no pudo completar la recomendación
            And no persiste parcialmente el nuevo mensaje, la respuesta, la evaluación ni sus recomendaciones
            And la conversación puede volver a procesarse cuando la IA de recomendación esté disponible
