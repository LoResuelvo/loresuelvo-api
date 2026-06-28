#!/usr/bin/env python3
"""Generate fake provider seed data and a matching asset upload manifest."""

from __future__ import annotations

import argparse
import hashlib
import re
import unicodedata
import uuid
from pathlib import Path


CATEGORIES = [
    "Plomería",
    "Electricidad",
    "Gas",
    "Carpintería",
    "Pintura",
    "Climatización",
]

FIRST_NAMES = [
    "Juan",
    "Laura",
    "Carlos",
    "María",
    "Pedro",
    "Sofía",
    "Diego",
    "Valentina",
    "Martín",
    "Camila",
    "Nicolás",
    "Agustina",
    "Federico",
    "Lucía",
    "Gonzalo",
    "Florencia",
    "Matías",
    "Julieta",
    "Santiago",
    "Carolina",
]

SURNAMES = [
    "Gómez",
    "Pérez",
    "Rodríguez",
    "Fernández",
    "López",
    "Martínez",
    "García",
    "Sánchez",
    "Romero",
    "Díaz",
    "Torres",
    "Álvarez",
    "Ruiz",
    "Silva",
    "Castro",
    "Molina",
    "Ramos",
    "Vega",
    "Herrera",
    "Medina",
]

CATEGORY_COLORS = {
    "plomeria": ("#1976d2", "#e3f2fd"),
    "electricidad": ("#f9a825", "#fff8e1"),
    "gasista": ("#ef6c00", "#fff3e0"),
    "carpinteria": ("#8d6e63", "#efebe9"),
    "pintura": ("#7b1fa2", "#f3e5f5"),
    "cerrajeria": ("#455a64", "#eceff1"),
    "jardineria": ("#2e7d32", "#e8f5e9"),
    "albanileria": ("#c62828", "#ffebee"),
    "climatizacion": ("#00838f", "#e0f7fa"),
    "techista": ("#5d4037", "#efebe9"),
}


def main() -> None:
    args = parse_args()
    categories = parse_categories(args.categories)
    providers = build_providers(args.count, categories, args.asset_variants)

    write_seed(args.output, categories, providers)
    write_manifest(args.manifest, providers)
    if args.assets_dir:
        write_assets(args.assets_dir, categories, args.asset_variants)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--count", type=int, default=100, help="number of providers to generate")
    parser.add_argument("--output", type=Path, default=Path("seeds/providers-100.yaml"))
    parser.add_argument("--manifest", type=Path, default=Path("seeds/providers-100-assets.tsv"))
    parser.add_argument(
        "--categories",
        default=",".join(CATEGORIES),
        help="comma-separated category names",
    )
    parser.add_argument("--asset-variants", type=int, default=3, help="base image variants per category")
    parser.add_argument(
        "--assets-dir",
        type=Path,
        default=Path("seeds/assets/provider_profile_photo"),
        help="directory where reusable WebP seed assets will be generated",
    )
    args = parser.parse_args()

    if args.count <= 0:
        parser.error("--count must be positive")
    if args.asset_variants <= 0:
        parser.error("--asset-variants must be positive")

    return args


def parse_categories(raw_categories: str) -> list[str]:
    categories = [category.strip() for category in raw_categories.split(",") if category.strip()]
    if not categories:
        raise SystemExit("at least one category is required")
    return categories


def build_providers(count: int, categories: list[str], asset_variants: int) -> list[dict[str, str]]:
    providers = []
    for index in range(1, count + 1):
        category = categories[(index - 1) % len(categories)]
        category_slug = slugify(category)
        name = FIRST_NAMES[(index - 1) % len(FIRST_NAMES)]
        surname = SURNAMES[((index - 1) // len(FIRST_NAMES) + index - 1) % len(SURNAMES)]
        provider_code = f"{index:04d}"
        asset_variant = ((index - 1) % asset_variants) + 1
        file_name = f"provider-{provider_code}.webp"

        providers.append(
            {
                "auth_id": f"auth0|seed-provider-{provider_code}",
                "email": f"{slugify(name)}.{slugify(surname)}.{provider_code}@seed.loresuelvo.local",
                "name": name,
                "surname": surname,
                "category": category,
                "profile_photo_file_id": str(uuid.uuid5(uuid.NAMESPACE_DNS, f"loresuelvo-seed-provider-{provider_code}")),
                "profile_photo_name": file_name,
                "profile_photo_key": f"seed/provider_profile_photo/providers-100/{file_name}",
                "asset_source": f"provider_profile_photo/{category_slug}-{asset_variant:02d}.webp",
            }
        )
    return providers


def write_seed(path: Path, categories: list[str], providers: list[dict[str, str]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as seed_file:
        seed_file.write("categories:\n")
        for category in categories:
            seed_file.write(f"  - name: {yaml_string(category)}\n")
        seed_file.write("\nproviders:\n")
        for provider in providers:
            seed_file.write(f"  - auth_id: {yaml_string(provider['auth_id'])}\n")
            seed_file.write(f"    email: {yaml_string(provider['email'])}\n")
            seed_file.write(f"    name: {yaml_string(provider['name'])}\n")
            seed_file.write(f"    surname: {yaml_string(provider['surname'])}\n")
            seed_file.write(f"    category: {yaml_string(provider['category'])}\n")
            seed_file.write(f"    profile_photo_file_id: {provider['profile_photo_file_id']}\n")
            seed_file.write(f"    profile_photo_name: {yaml_string(provider['profile_photo_name'])}\n")
            seed_file.write(f"    profile_photo_key: {yaml_string(provider['profile_photo_key'])}\n")


def write_manifest(path: Path, providers: list[dict[str, str]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as manifest_file:
        manifest_file.write("# source_asset_relative_to_seeds_assets\ttarget_object_key\n")
        for provider in providers:
            manifest_file.write(f"{provider['asset_source']}\t{provider['profile_photo_key']}\n")


def write_assets(path: Path, categories: list[str], variants: int) -> None:
    try:
        from PIL import Image, ImageDraw, ImageFont
    except ImportError as exc:
        raise SystemExit("Pillow is required to generate WebP assets") from exc

    path.mkdir(parents=True, exist_ok=True)
    font = ImageFont.load_default()

    for category in categories:
        category_slug = slugify(category)
        foreground, background = CATEGORY_COLORS.get(category_slug, color_pair_for(category_slug))
        for variant in range(1, variants + 1):
            image = Image.new("RGB", (512, 512), background)
            draw = ImageDraw.Draw(image)
            draw.ellipse((96, 64, 416, 384), fill=foreground)
            draw.rectangle((176, 320, 336, 448), fill=foreground)
            draw.ellipse((184, 120, 328, 264), fill="#f5c7a9")
            draw.rectangle((224, 264, 288, 352), fill="#f5c7a9")
            draw.rectangle((160, 368, 352, 448), fill=shift_color(foreground, variant * 18))
            initials = initials_for(category)
            text_box = draw.textbbox((0, 0), initials, font=font)
            text_width = text_box[2] - text_box[0]
            draw.text(((512 - text_width) / 2, 458), initials, fill="#263238", font=font)
            image.save(path / f"{category_slug}-{variant:02d}.webp", "WEBP", quality=82, method=6)


def color_pair_for(value: str) -> tuple[str, str]:
    digest = hashlib.sha256(value.encode("utf-8")).digest()
    foreground = f"#{digest[0]:02x}{digest[1]:02x}{digest[2]:02x}"
    background = f"#{220 + digest[3] % 25:02x}{220 + digest[4] % 25:02x}{220 + digest[5] % 25:02x}"
    return foreground, background


def shift_color(hex_color: str, amount: int) -> str:
    red = min(255, int(hex_color[1:3], 16) + amount)
    green = min(255, int(hex_color[3:5], 16) + amount)
    blue = min(255, int(hex_color[5:7], 16) + amount)
    return f"#{red:02x}{green:02x}{blue:02x}"


def initials_for(category: str) -> str:
    words = [word for word in re.split(r"\s+", category.strip()) if word]
    return "".join(word[0].upper() for word in words[:2])


def slugify(value: str) -> str:
    normalized = unicodedata.normalize("NFKD", value)
    ascii_value = normalized.encode("ascii", "ignore").decode("ascii")
    slug = re.sub(r"[^a-zA-Z0-9]+", "-", ascii_value.lower()).strip("-")
    return slug or "seed"


def yaml_string(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


if __name__ == "__main__":
    main()
