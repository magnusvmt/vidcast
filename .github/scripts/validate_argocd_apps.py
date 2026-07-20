#!/usr/bin/env python3
"""Structural check for the Argo CD Application manifests under deploy/argocd/.

Mirrors what `helm lint`/`terraform validate` do for the other deploy
artifacts: catch malformed YAML and missing required fields without needing
a live cluster to apply against.
"""
import glob
import sys

import yaml

REQUIRED_PATHS = [
    ("metadata", "name"),
    ("spec", "source", "repoURL"),
    ("spec", "source", "path"),
    ("spec", "destination", "server"),
    ("spec", "destination", "namespace"),
]


def check(path: str) -> bool:
    with open(path) as f:
        doc = yaml.safe_load(f)

    if not isinstance(doc, dict):
        print(f"{path}: not a valid YAML mapping (empty or malformed)")
        return False

    ok = True
    if doc.get("apiVersion") != "argoproj.io/v1alpha1" or doc.get("kind") != "Application":
        print(f"{path}: apiVersion/kind must be argoproj.io/v1alpha1 Application")
        return False

    for field_path in REQUIRED_PATHS:
        node = doc
        for key in field_path:
            if not isinstance(node, dict) or key not in node:
                print(f"{path}: missing {'.'.join(field_path)}")
                ok = False
                break
            node = node[key]

    return ok


def main() -> int:
    files = ["deploy/argocd/root-app.yaml"] + sorted(glob.glob("deploy/argocd/apps/*.yaml"))
    ok = True
    for f in files:
        if check(f):
            print(f"{f}: OK")
        else:
            ok = False
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
