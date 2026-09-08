# LoResuelvo US-60 — extensión visual derivada

Selección de cuatro casos visuales (`PD-049`–`PD-052`) para desarrollo; `PD-051` también integra `critical_all`. Las imágenes son sintéticas, generadas con `built-in image_gen`, y no representan escenas reales.

Fuente: `LoResuelvo_US60_evals_v1.0.0`, cuyo `manifest.json` tiene SHA-256 `20de062181f7f1bcc99d92cf0fb1bdd9587456d03f516becfae66773b46c0c97`. Las filas originales permanecen sin cambios; los cuatro casos son clones adaptados de `PD-011`, `PD-020`, `PD-030` y `PD-038`, con límites explícitos sobre lo observable en cada fotografía.

Validación y selección offline:

```bash
go run ./cmd/evals validate --dataset evals/datasets/LoResuelvo_US60_visual_v1.0.0
go run ./cmd/evals plan --dataset evals/datasets/LoResuelvo_US60_visual_v1.0.0 --suite development --cases PD-049,PD-050,PD-051,PD-052 --trials 3 --model gemini-3.5-flash-lite --max-requests 12
```

El ejecutor Python del paquete padre también valida este derivado mediante `validate(root)`; no se copia al paquete nuevo.
