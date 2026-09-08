# LoResuelvo US-60 — Evaluaciones 1.1.0

Amplía el dataset `1.0.0` con cuatro casos visuales sintéticos (`PD-049`–`PD-052`) para desarrollo; `PD-051` también integra `critical_all`. Contiene 52 casos de prediagnóstico, 24 de ranking y 12 contratos de servicio.

Los casos añadidos adaptan `PD-011`, `PD-020`, `PD-030` y `PD-038`, con límites explícitos sobre lo observable. Sus imágenes fueron generadas con `built-in image_gen` y no representan escenas reales. Los materiales del dataset base se conservan sin cambios.

## Validación

Desde la raíz del repositorio:

```bash
go run ./cmd/evals validate --dataset evals/datasets/LoResuelvo_US60_evals_v1.1.0
```

`manifest.json` contiene el inventario y los hashes SHA-256 de los archivos, e identifica el dataset padre.

## Política de versiones

- **Documentación y empaquetado**: las correcciones editoriales que no alteran los materiales evaluados pueden mantenerse en la misma versión, actualizando los hashes afectados y registrando el cambio en Git.
- **Patch (`1.1.x`)**: correcciones compatibles que requieren una nueva publicación sin cambiar el significado de los casos ni los criterios.
- **Minor (`1.x.0`)**: incorporación compatible de casos, assets, criterios o campos opcionales.
- **Major (`x.0.0`)**: cambios incompatibles de esquema o contrato, o que alteren el significado de casos o criterios existentes.

Los hashes de informes y configuraciones de medición identifican los archivos utilizados en esa ejecución y no se reescriben por correcciones editoriales posteriores.
