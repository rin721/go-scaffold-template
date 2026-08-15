[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$versionFile = Join-Path $PSScriptRoot 'tool-versions.env'
$versions = @{}
foreach ($line in Get-Content -LiteralPath $versionFile -Encoding utf8) {
    if ($line -match '^([A-Z0-9_]+)=(.+)$') {
        $versions[$Matches[1]] = $Matches[2]
    }
}

$toolDirectory = Join-Path $repositoryRoot '.tools\bin'
New-Item -ItemType Directory -Force -Path $toolDirectory | Out-Null
$previousGoBin = $env:GOBIN
try {
    $env:GOBIN = $toolDirectory
    go install "github.com/goreleaser/goreleaser/v2@$($versions.GORELEASER_VERSION)"
    go install "github.com/anchore/syft/cmd/syft@$($versions.SYFT_VERSION)"
    go install "github.com/sigstore/cosign/v2/cmd/cosign@$($versions.COSIGN_VERSION)"
    go install "golang.org/x/vuln/cmd/govulncheck@$($versions.GOVULNCHECK_VERSION)"
    go install "github.com/securego/gosec/v2/cmd/gosec@$($versions.GOSEC_VERSION)"
    go install "github.com/zricethezav/gitleaks/v8@$($versions.GITLEAKS_VERSION)"
} finally {
    $env:GOBIN = $previousGoBin
}
