#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tool_dir="${repository_root}/.tools/bin"
export PATH="${tool_dir}:${PATH}"
export SYFT_CHECK_FOR_APP_UPDATE=false
cd "${repository_root}"

for tool in goreleaser syft cosign; do
  command -v "${tool}" >/dev/null || { echo "missing ${tool}; run scripts/install-tools.sh" >&2; exit 1; }
done

goreleaser release --snapshot --clean --skip=sign,publish,announce

temporary_key_dir="$(mktemp -d)"
cleanup() { rm -rf "${temporary_key_dir}"; }
trap cleanup EXIT
export COSIGN_PASSWORD="$(printf '%s' "${RANDOM}-${RANDOM}-${RANDOM}-${RANDOM}" | sha256sum | cut -d' ' -f1)"
cosign generate-key-pair --output-key-prefix "${temporary_key_dir}/local-rc" >/dev/null
cosign sign-blob --yes --tlog-upload=false \
  --key "${temporary_key_dir}/local-rc.key" \
  --bundle dist/checksums.txt.bundle \
  --output-signature dist/checksums.txt.sig \
  dist/checksums.txt
cp "${temporary_key_dir}/local-rc.pub" dist/local-rc.pub
cosign verify-blob --insecure-ignore-tlog \
  --key dist/local-rc.pub \
  --bundle dist/checksums.txt.bundle \
  --signature dist/checksums.txt.sig \
  dist/checksums.txt

(cd dist && sha256sum --check checksums.txt)
