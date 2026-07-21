#!/usr/bin/env python3
"""Fail when a typed public backend route is absent from OpenAPI."""

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
route_pattern = re.compile(
    r'(?:HandleFunc|Handle)\(\s*"(?:(GET|POST|PUT|PATCH|DELETE) )(/api/[^" ]+)'
)
routes: set[tuple[str, str]] = set()
for source in (ROOT / "back" / "internal").rglob("*.go"):
    for method, path in route_pattern.findall(source.read_text(errors="ignore")):
        routes.add((method.lower(), path))

documented: set[tuple[str, str]] = set()
current_path = ""
for line in (ROOT / "openapi.yaml").read_text().splitlines():
    path_match = re.match(r"^  (/[^:]+):\s*$", line)
    if path_match:
        current_path = path_match.group(1)
        continue
    operation_match = re.match(r"^    (get|post|put|patch|delete):\s*$", line)
    if operation_match and current_path:
        documented.add((operation_match.group(1), current_path))

missing = sorted(routes - documented)
extra = sorted(documented - routes)
print(f"backend={len(routes)} openapi={len(documented)} missing={len(missing)} extra={len(extra)}")
if missing:
    for method, path in missing:
        print(f"MISSING {method.upper()} {path}")
    sys.exit(1)
