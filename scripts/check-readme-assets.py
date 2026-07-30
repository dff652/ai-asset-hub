#!/usr/bin/env python3
"""Validate repository-owned README SVG and embed invariants."""

from __future__ import annotations

import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
ASSET_DIR = ROOT / "assets" / "readme"
EXPECTED_DIMENSIONS = {
    "hero.svg": (1200, 440),
    "usage-flow.svg": (1200, 600),
    "asset-lifecycle.svg": (1200, 620),
    "tui-proof-board.svg": (1200, 1000),
}
EMBED_DOCUMENTS = (ROOT / "README.md", ROOT / "docs" / "getting-started.md")
MIN_FONT_SIZE = 16
MAX_ASSET_BYTES = 100_000
SVG_NAMESPACE = "http://www.w3.org/2000/svg"
XLINK_NAMESPACE = "http://www.w3.org/1999/xlink"
SYSTEM_FONT_PREFIXES = {
    "-apple-system",
    "BlinkMacSystemFont",
    "SFMono-Regular",
    "system-ui",
    "ui-monospace",
}


class CheckError(Exception):
    pass


def local_name(tag: str) -> str:
    return tag.rsplit("}", 1)[-1]


def require(condition: bool, message: str) -> None:
    if not condition:
        raise CheckError(message)


def check_text_nodes(
    element: ET.Element,
    inherited_font: str = "",
    inherited_size: str = "",
) -> None:
    font = element.attrib.get("font-family", inherited_font)
    raw_size = element.attrib.get("font-size", inherited_size)
    if local_name(element.tag) == "text":
        require(bool(font), "text node has no font-family or inherited font")
        require(
            "sans-serif" in font or "monospace" in font,
            f"text node uses a non-system font stack: {font}",
        )
        first_font = font.split(",", 1)[0].strip(" '\"")
        require(
            first_font in SYSTEM_FONT_PREFIXES,
            f"text node puts a non-system font first: {font}",
        )
        require(bool(raw_size), "text node has no explicit font-size")
        try:
            size = float(raw_size)
        except ValueError as error:
            raise CheckError(f"invalid font-size: {raw_size}") from error
        require(
            size >= MIN_FONT_SIZE,
            f"text node uses font-size {raw_size}; minimum is {MIN_FONT_SIZE}",
        )
    for child in element:
        check_text_nodes(child, font, raw_size)


def check_svg(path: Path, dimensions: tuple[int, int]) -> None:
    require(path.is_file(), f"missing SVG asset: {path.relative_to(ROOT)}")
    require(
        path.stat().st_size <= MAX_ASSET_BYTES,
        f"SVG exceeds {MAX_ASSET_BYTES} bytes: {path.relative_to(ROOT)}",
    )
    source = path.read_text(encoding="utf-8")
    require(
        re.search(r"url\(\s*['\"]?(?:https?:)?//", source, flags=re.IGNORECASE) is None,
        f"{path.name}: CSS contains a remote URL",
    )
    require("@import" not in source.lower(), f"{path.name}: CSS import is forbidden")
    try:
        root = ET.fromstring(source)
    except ET.ParseError as error:
        raise CheckError(f"invalid SVG XML in {path.relative_to(ROOT)}: {error}") from error

    width, height = dimensions
    require(local_name(root.tag) == "svg", f"{path.name}: root is not svg")
    require(root.attrib.get("width") == str(width), f"{path.name}: width must be {width}")
    require(root.attrib.get("height") == str(height), f"{path.name}: height must be {height}")
    require(
        root.attrib.get("viewBox") == f"0 0 {width} {height}",
        f"{path.name}: viewBox must match width and height",
    )
    require(root.attrib.get("role") == "img", f"{path.name}: role must be img")
    require(
        root.attrib.get("aria-labelledby") == "title desc",
        f"{path.name}: aria-labelledby must be 'title desc'",
    )

    title = root.find(f"{{{SVG_NAMESPACE}}}title")
    description = root.find(f"{{{SVG_NAMESPACE}}}desc")
    require(title is not None and bool((title.text or "").strip()), f"{path.name}: missing title")
    require(
        description is not None and bool((description.text or "").strip()),
        f"{path.name}: missing description",
    )

    for element in root.iter():
        name = local_name(element.tag)
        require(name not in {"script", "foreignObject"}, f"{path.name}: forbidden <{name}>")
        for key in ("href", f"{{{XLINK_NAMESPACE}}}href"):
            reference = element.attrib.get(key, "")
            require(
                not reference.startswith(("http://", "https://", "//")),
                f"{path.name}: remote resource {reference}",
            )
    check_text_nodes(root)


def check_embeds(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    for image_tag in re.findall(r"<img\\b[^>]*>", text, flags=re.IGNORECASE):
        source_match = re.search(r'\\bsrc="([^"]+)"', image_tag, flags=re.IGNORECASE)
        alt_match = re.search(r'\\balt="([^"]*)"', image_tag, flags=re.IGNORECASE)
        require(source_match is not None, f"{path.relative_to(ROOT)}: img has no src")
        require(
            alt_match is not None and bool(alt_match.group(1).strip()),
            f"{path.relative_to(ROOT)}: img has no meaningful alt",
        )
        source = source_match.group(1)
        if source.startswith(("http://", "https://")):
            continue
        target = (path.parent / source).resolve()
        require(target.is_file(), f"{path.relative_to(ROOT)}: missing image {source}")


def main() -> int:
    try:
        for name, dimensions in EXPECTED_DIMENSIONS.items():
            check_svg(ASSET_DIR / name, dimensions)
        for document in EMBED_DOCUMENTS:
            check_embeds(document)

        hero = (ASSET_DIR / "hero.svg").read_text(encoding="utf-8")
        require("aiah ui" not in hero, "hero must show the primary `aiah` command")
        require(">aiah</text>" in hero, "hero does not show the primary `aiah` command")

        proof = (ASSET_DIR / "tui-proof-board.svg").read_text(encoding="utf-8")
        require("DEV CANDIDATE" not in proof, "proof board still claims dev candidate")
        require("v0.1.6" in proof, "proof board does not identify its accepted release")
    except CheckError as error:
        print(f"readme assets: ERROR: {error}", file=sys.stderr)
        return 1

    print(
        "readme assets: OK "
        f"({len(EXPECTED_DIMENSIONS)} SVGs, {len(EMBED_DOCUMENTS)} embed documents)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
