#!/usr/bin/env python3
"""Bundle the modular OpenAPI source into a single JSON artifact.

The source spec uses file-based $ref values for path items, security schemes,
and schemas. This bundler resolves those file references while preserving
internal references such as #/components/schemas/ErrorResponse, which keeps the
published JSON readable and compatible with common tooling.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import yaml


def load_yaml(path: Path) -> Any:
    with path.open(encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def resolve_pointer(document: Any, pointer: str) -> Any:
    if pointer in ("", "#"):
        return document
    if pointer.startswith("#"):
        pointer = pointer[1:]
    if not pointer.startswith("/"):
        raise ValueError(f"Unsupported JSON pointer: {pointer!r}")

    current = document
    for raw_part in pointer.lstrip("/").split("/"):
        part = raw_part.replace("~1", "/").replace("~0", "~")
        if isinstance(current, list):
            current = current[int(part)]
        else:
            current = current[part]
    return current


def bundle_node(node: Any, current_file: Path) -> Any:
    if isinstance(node, list):
        return [bundle_node(item, current_file) for item in node]

    if not isinstance(node, dict):
        return node

    ref = node.get("$ref")
    if isinstance(ref, str):
        # Preserve internal references to keep schemas reusable in the bundled JSON.
        if ref.startswith("#"):
            return dict(node)

        file_part, _, pointer = ref.partition("#")
        ref_file = (current_file.parent / file_part).resolve()
        ref_document = load_yaml(ref_file)
        resolved = resolve_pointer(ref_document, f"#{pointer}" if pointer else "#")
        return bundle_node(resolved, ref_file)

    return {key: bundle_node(value, current_file) for key, value in node.items()}


def main() -> None:
    parser = argparse.ArgumentParser(description="Bundle modular OpenAPI YAML into JSON")
    parser.add_argument("--input", default="openapi/openapi.yaml", help="Root OpenAPI YAML file")
    parser.add_argument("--output", default="openapi.json", help="Bundled JSON output file")
    args = parser.parse_args()

    input_path = Path(args.input).resolve()
    output_path = Path(args.output)
    spec = bundle_node(load_yaml(input_path), input_path)

    output_path.write_text(
        json.dumps(spec, ensure_ascii=False, indent=4) + "\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
