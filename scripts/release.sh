#!/usr/bin/env bash

set -euo pipefail

git fetch --tags --force

# 2. Obtener el tag más alto que coincida con el patrón v*.*.*
# - git tag -l "v*.*.*": Lista solo los tags con formato v1.2.3
# - sort -V: Ordena versiones de forma lógica (ej: v1.10.0 es mayor que v1.2.0)
# - tail -n1: Se queda con la última (la más alta)
LATEST_TAG=$(git tag -l | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n1 || true)

if [ -z "$LATEST_TAG" ]; then
    LATEST_TAG="v0.0.0"
fi

echo "Last registered tag: $LATEST_TAG"

VERSION_NUMBERS="${LATEST_TAG#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION_NUMBERS"

# Por el momento solo aumentamos el patch
NEW_PATCH=$((PATCH + 1))
NEW_TAG="v${MAJOR}.${MINOR}.${NEW_PATCH}"

echo "New tag will be: $NEW_TAG"

# Pregunta final para estar seguros
read -p "Create and push tag $NEW_TAG ? (y/n): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Ss]$ ]]; then
    exit 1
fi

git tag "$NEW_TAG"
git push origin "$NEW_TAG"