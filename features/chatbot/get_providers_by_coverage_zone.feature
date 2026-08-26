Feature: Recomendar prestadores elegibles por zona de cobertura
    Como consumidor
    quiero recibir únicamente prestadores que cubran la zona de mi domicilio
    para contactar profesionales que realmente puedan atenderme

    Background:
        Given que existe el rubro "Plomería"
        And que están habilitadas las zonas de cobertura "Comuna 6" y "Comuna 14"
        And que existe un consumidor registrado con correo "ana@example.com", nombre "Ana" y apellido "Pérez"
        And que estoy autenticado como consumidor "ana@example.com"
        And que el chatbot asistido por IA está disponible

    Rule: Un prestador del rubro diagnosticado solo es elegible si cubre la zona del consumidor

        Scenario: 13.2.1-GP Recomendar únicamente prestadores que cubren la zona del consumidor
            Given que el domicilio del consumidor "ana@example.com" pertenece a la zona de cobertura "Comuna 6"
            And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y zona de cobertura "Comuna 6"
            And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 14"
            And que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "Plomería" con la respuesta:
                """
                El problema parece ser una pérdida de agua. Te recomiendo contactar a un plomero.
                """
            When envío un mensaje al chatbot asistido por IA:
                """
                Pierde agua la conexión bajo la pileta de la cocina.
                """
            Then el sistema muestra al prestador recomendado "Juan Gómez" en la respuesta del chatbot
            And el sistema no muestra al prestador recomendado "Pedro Dib" en la respuesta del chatbot

        Scenario: 13.2.2-GP Recomendar un prestador que cubre varias zonas incluida la del consumidor
            Given que el domicilio del consumidor "ana@example.com" pertenece a la zona de cobertura "Comuna 14"
            And existe un prestador registrado con correo "juan.plomero@example.com", nombre "Juan", apellido "Gómez", rubro "Plomería" y las zonas de cobertura "Comuna 6" y "Comuna 14"
            And que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "Plomería" con la respuesta:
                """
                El problema parece ser una pérdida de agua. Te recomiendo contactar a un plomero.
                """
            When envío un mensaje al chatbot asistido por IA:
                """
                Pierde agua la conexión bajo la pileta de la cocina.
                """
            Then el sistema muestra al prestador recomendado "Juan Gómez" en la respuesta del chatbot

        Scenario: 13.2.3-GP Mostrar una lista vacía cuando ningún prestador del rubro cubre la zona del consumidor
            Given que el domicilio del consumidor "ana@example.com" pertenece a la zona de cobertura "Comuna 6"
            And existe un prestador registrado con correo "pedro.plomero@example.com", nombre "Pedro", apellido "Dib", rubro "Plomería" y zona de cobertura "Comuna 14"
            And que el chatbot asistido por IA concluirá el diagnóstico y recomendará el rubro "Plomería" con la respuesta:
                """
                El problema parece ser una pérdida de agua. Te recomiendo contactar a un plomero.
                """
            When envío un mensaje al chatbot asistido por IA:
                """
                Pierde agua la conexión bajo la pileta de la cocina.
                """
            Then el sistema muestra una lista vacía de prestadores recomendados en la respuesta del chatbot
