@wip @US-20
Feature: Recibir avisos sobre mis servicios
    Como consumidor o prestador
    quiero recibir avisos en mi teléfono aunque no esté usando LoResuelvo
    para enterarme de los mensajes y las novedades de mis servicios

    Background:
        Given que Ana es una consumidora y Juan es un prestador registrados
        And que ambos tienen un teléfono registrado para recibir avisos

    Scenario Outline: 20.1 Avisar un mensaje nuevo solamente a quien lo recibe
        Given que Ana y Juan pueden conversar sobre un trabajo
        And que <destinatario> no está usando la aplicación
        When <autor> envía un mensaje <contenido>
        Then LoResuelvo envía al teléfono de <destinatario> un aviso de nuevo mensaje que indica a qué conversación pertenece
        And no envía ese aviso al autor ni a personas ajenas a la conversación

        Examples:
            | autor | destinatario | contenido           |
            | Ana   | Juan         | de texto            |
            | Juan  | Ana          | con varias imágenes |
            | Ana   | Juan         | de audio            |
            | Juan  | Ana          | con video           |

    Scenario: 20.2 Avisar al consumidor que recibió una propuesta
        Given que Juan puede ofrecerle un servicio a Ana
        When Juan le envía una propuesta válida
        Then LoResuelvo envía solamente a Ana un aviso de nueva propuesta
        And el aviso indica cuál es la propuesta recibida

    Scenario: 20.3 Avisar al prestador que se confirmó una contratación
        Given que Ana inició el pago de la seña de una propuesta vigente de Juan
        When LoResuelvo verifica que la seña fue aprobada y confirma la contratación
        Then LoResuelvo envía solamente a Juan un aviso de propuesta aceptada
        And el aviso indica cuál es la orden de trabajo generada

    Scenario: 20.4 Recordar el turno a ambos participantes sin repetir el aviso
        Given que Ana y Juan tienen un trabajo programado para dentro de 23 horas
        When LoResuelvo revisa los próximos turnos más de una vez
        Then envía a Ana y a Juan un único recordatorio a cada uno sobre ese trabajo

    Scenario: 20.5 Avisar al consumidor que el prestador terminó el trabajo
        Given que Juan tiene un trabajo de Ana pendiente de finalizar
        And que ya llegó la fecha acordada para realizarlo
        When Juan informa que terminó el trabajo con una descripción y fotos válidas
        Then LoResuelvo envía solamente a Ana un aviso de trabajo finalizado
        And el aviso indica cuál es el trabajo que puede revisar

    Scenario: 20.6 Avisar solamente al prestador que se aprobó el pago final
        Given que Juan informó la finalización del trabajo de Ana con las fotos requeridas
        And que Ana inició el pago del saldo de ese trabajo
        When LoResuelvo verifica que el pago fue aprobado y registra el trabajo como pagado
        Then LoResuelvo envía solamente a Juan un aviso de pago final confirmado
        And el aviso indica cuál es el trabajo pagado

    Scenario Outline: 20.7 No anunciar un pago final que todavía no está confirmado
        Given que Ana inició el pago del saldo de un trabajo realizado por Juan
        When <situacion>
        Then LoResuelvo no envía un aviso de pago final confirmado

        Examples:
            | situacion                                                  |
            | el pago queda pendiente                                    |
            | el pago es rechazado                                       |
            | Ana vuelve del sitio de pago sin que se verifique el cobro  |

    Scenario: 20.8 No repetir el aviso por el mismo pago final
        Given que LoResuelvo ya confirmó el pago final de un trabajo de Juan y envió su aviso
        When vuelve a recibir la confirmación de ese mismo pago
        Then no envía otro aviso de pago final confirmado

    Scenario: 20.9 No avisar un mensaje que no se pudo enviar
        Given que Juan todavía no aceptó la solicitud de trabajo de Ana
        When Juan intenta enviarle un mensaje en esa conversación
        Then LoResuelvo rechaza el mensaje
        And no envía a Ana un aviso de nuevo mensaje

    Scenario: 20.10 Proteger el contenido privado de la conversación
        Given que Ana y Juan pueden conversar sobre un trabajo
        When Ana envía un mensaje con su nombre, dirección y fotos del domicilio
        Then el aviso para Juan solo informa que tiene un nuevo mensaje en LoResuelvo
        And no incluye el contenido del mensaje ni los enlaces a las fotos

    Scenario Outline: 20.11 Respetar el idioma elegido para los avisos
        Given que el teléfono de Ana está registrado <preferencia>
        And que Juan puede ofrecerle un servicio a Ana
        When Juan le envía una propuesta válida
        Then el aviso de nueva propuesta para Ana está escrito en <idioma>

        Examples:
            | preferencia           | idioma  |
            | en español            | español |
            | en inglés             | inglés  |
            | sin elegir un idioma  | español |
