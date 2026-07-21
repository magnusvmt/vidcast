#!/usr/bin/env python3
"""Rewrite the `tag:` value inside an `image:` block in a YAML file in place.

Used by the CI GitOps image-bump job: after publishing a service's image to
GHCR, this updates that service's deploy manifest to point at the new tag
(rather than CI running `helm upgrade` against the cluster directly), so
ArgoCD picks up the change on its next sync.

Deliberately does a targeted text replacement instead of a full YAML
load/dump round-trip, since a round-trip would drop the explanatory comments
that live in these files.
"""
import sys


def bump_tag(text: str, new_tag: str) -> str:
    lines = text.splitlines(keepends=True)
    image_indent = None
    for i, line in enumerate(lines):
        stripped = line.strip()
        indent = len(line) - len(line.lstrip(" "))

        if image_indent is not None and stripped and indent <= image_indent:
            # Dedented back out of the image: block without finding tag.
            image_indent = None

        if stripped == "image:":
            image_indent = indent
            continue

        if image_indent is None or not stripped:
            continue

        if stripped.startswith("tag:"):
            prefix = line[: len(line) - len(line.lstrip())]
            lines[i] = f"{prefix}tag: {new_tag}\n"
            return "".join(lines)

    raise ValueError("no image.tag field found")


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print(f"usage: {argv[0]} <yaml-file> <new-tag>", file=sys.stderr)
        return 2

    path, new_tag = argv[1], argv[2]
    with open(path) as f:
        text = f.read()

    with open(path, "w") as f:
        f.write(bump_tag(text, new_tag))

    print(f"{path}: tag -> {new_tag}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
