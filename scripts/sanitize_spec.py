#!/usr/bin/env python3
"""Strip `example`/`examples` keys from an OpenAPI spec before codegen.

Some RBHU specs put `example` `$ref`s that point into `components/schemas`
instead of `components/examples`, which the strict OpenAPI loader rejects.
Examples never affect generated types or clients, so we drop them.

Usage: sanitize_spec.py <in.yaml> <out.yaml>
"""
import sys
import yaml


def strip(node):
    if isinstance(node, dict):
        return {k: strip(v) for k, v in node.items() if k not in ("example", "examples")}
    if isinstance(node, list):
        return [strip(v) for v in node]
    return node


def main():
    src, dst = sys.argv[1], sys.argv[2]
    with open(src) as f:
        spec = yaml.safe_load(f)
    with open(dst, "w") as f:
        yaml.safe_dump(strip(spec), f, sort_keys=False, allow_unicode=True)


if __name__ == "__main__":
    main()
