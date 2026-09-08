# LoResuelvo US-60 — extensión visual publicada 1.1.0

Esta carpeta publica una copia inmutable de `LoResuelvo_US60_visual_v1.0.0`, cuyo manifiesto tenía la versión exacta `1.0.0-visual.2`. Su dataset padre es `LoResuelvo_US60_evals_v1.0.0` (versión `1.0.0`). Las filas de casos, assets, configuraciones, políticas y esquemas se conservaron byte por byte; esta publicación no corrige ni reinterpreta esos materiales.

La diferencia de esta publicación es de empaquetado y procedencia: `manifest.json` identifica explícitamente la fuente visual, el padre y los hashes de esta copia. No contiene resultados de modelos ni llamadas en vivo.

## Validación

Desde la raíz del repositorio:

```bash
go run ./cmd/evals validate --dataset evals/datasets/LoResuelvo_US60_evals_v1.1.0
```

La equivalencia se verifica comparando los hashes de `files_sha256` de ambos manifiestos, excluyendo `README.md`.

## Política de versiones

- **Patch (`1.1.x`)**: correcciones de documentación, metadatos o integridad que no cambian casos, criterios, contratos, esquemas ni el significado de los datos. Se regeneran los hashes afectados.
- **Minor (`1.x.0`)**: incorporación compatible de casos, assets, criterios o campos opcionales que no invalida consumidores ni cambia el significado de los materiales existentes. Se conserva la procedencia y se documenta la ampliación.
- **Major (`x.0.0`)**: cambios incompatibles de esquema o contrato, eliminación o renombrado de campos/casos, o cambios que alteren el significado de criterios, resultados esperados o materiales existentes.

Las copias publicadas son inmutables: cualquier cambio requiere una nueva versión y un nuevo manifiesto; no se modifica `LoResuelvo_US60_visual_v1.0.0`.
