#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "${repository_root}/scripts/tool-versions.env"

tool_dir="${repository_root}/.tools/bin"
mkdir -p "${tool_dir}"
export GOBIN="${tool_dir}"

go install "github.com/goreleaser/goreleaser/v2@${GORELEASER_VERSION}"
go install "github.com/anchore/syft/cmd/syft@${SYFT_VERSION}"
go install "github.com/sigstore/cosign/v2/cmd/cosign@${COSIGN_VERSION}"
go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
go install "github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}"
go install "github.com/zricethezav/gitleaks/v8@${GITLEAKS_VERSION}"
