Feature: Recomendaciones de Prestadores por IA
    Como consumidor
    quiero recibir recomendaciones de prestadores por parte de un chatbot asistido por IA
    para encontrar profesionales que puedan ayudarme con el problema de mi hogar

    Background:
        Given que existe el rubro "Plomería"
        And que existe el rubro "Electricidad"
        And que existe el rubro "Gasista"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez" y rubro "Plomería"
        And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib" y rubro "Plomería"
        And existe un prestador registrado con correo "laura.electricista@example.com", nombre "Laura", apellido "Suárez" y rubro "Electricidad"
        And que el chatbot asistido por IA está disponible

    Rule: Las recomendaciones se devuelven junto con la respuesta del chatbot cuando el diagnóstico está concluido

    Scenario: 13.1.1-GP - Cuando no hay prestadores disponibles se obtiene una lista vacía
        Given que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "Gasista" con la respuesta:
            """
            El problema parece corresponder a una pérdida de gas. Cerrá la llave de paso, ventilá el ambiente y contactá a un gasista matriculado.
            """
        When envío un mensaje al chatbot asistido por IA:
            """
            Siento olor a gas cerca de la cocina cuando abro la llave.
            """
        Then el sistema muestra la respuesta del chatbot asistido por IA:
            """
            El problema parece corresponder a una pérdida de gas. Cerrá la llave de paso, ventilá el ambiente y contactá a un gasista matriculado.
            """
        And el sistema muestra una lista vacía de prestadores recomendados en la respuesta del chatbot

    Scenario: 13.1.2-GP - Recibir recomendaciones de prestadores por parte del chatbot asistido por IA
        Given que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "Plomería" con la respuesta:
            """
            El problema parece ser una pérdida en el sifón o en una conexión flexible de la bacha. Te recomiendo contactar a un plomero.
            """
        When envío un mensaje al chatbot asistido por IA:
            """
            Hay agua acumulada dentro del mueble bajo mesada cada vez que uso la pileta.
            """
        Then el sistema muestra la respuesta del chatbot asistido por IA:
            """
            El problema parece ser una pérdida en el sifón o en una conexión flexible de la bacha. Te recomiendo contactar a un plomero.
            """
        And el sistema muestra al prestador recomendado "Juan Gómez" en la respuesta del chatbot
        And el sistema muestra al prestador recomendado "Pedro Dib" en la respuesta del chatbot
        And el sistema no muestra al prestador recomendado "Laura Suárez" en la respuesta del chatbot

    Scenario: 13.1.3-GP - No se recomiendan prestadores cuando el chatbot todavía no concluyó el diagnóstico
        Given que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA responderá:
            """
            Necesito algunos datos más: ¿el agua aparece cuando abrís la canilla o incluso con la canilla cerrada?
            """
        When envío un mensaje al chatbot asistido por IA:
            """
            Tengo humedad en la cocina, pero no sé de dónde viene.
            """
        Then el sistema muestra la respuesta del chatbot asistido por IA:
            """
            Necesito algunos datos más: ¿el agua aparece cuando abrís la canilla o incluso con la canilla cerrada?
            """
        And el sistema no muestra prestadores recomendados en la respuesta del chatbot
