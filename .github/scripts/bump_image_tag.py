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
    seen_image_block = False
    found_image_tag = False
    for i, line in enumerate(lines):
        stripped = line.strip()
        indent = len(line) - len(line.lstrip(" "))

        if image_indent is not None:
            if stripped and indent <= image_indent:
                # Dedented back out of the image: block.
                image_indent = None
            elif stripped.startswith("tag:"):
                if found_image_tag:
                    raise ValueError("multiple image.tag fields found")
                found_image_tag = True
                prefix = line[: len(line) - len(line.lstrip())]
                tag_content = stripped[len("tag:"):].strip()
                trailing_comment = ""
                hash_pos = tag_content.find("#")
                if hash_pos != -1:
                    trailing_comment = "  " + tag_content[hash_pos:].strip()
                lines[i] = f"{prefix}tag: {new_tag}{trailing_comment}\n"
                continue

        if stripped == "image:":
            if seen_image_block:
                raise ValueError("multiple image: blocks found; ambiguous which to patch")
            seen_image_block = True
            image_indent = indent

    if found_image_tag:
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
