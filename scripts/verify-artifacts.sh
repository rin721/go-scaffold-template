#!/usr/bin/env bash
set -euo pipefail

unexpected="$(git ls-files | grep -E '\.(exe|dll|so|dylib|test|out|db|sqlite|key|pem|p12|pfx|zip|tar|gz)$' || true)"
if [[ -n "${unexpected}" ]]; then
  printf 'tracked build, database, key, or archive artifacts are forbidden:\n%s\n' "${unexpected}" >&2
  exit 1
fi
