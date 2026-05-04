Feature: Saludo inicial
  Scenario: Obtener el saludo de bienvenida
    Given que el sistema esta iniciado
    When solicito el saludo en la ruta raiz
    Then la respuesta debe ser "Hello World" con un codigo 200