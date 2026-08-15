[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repositoryRoot
try {
    $unformatted = @(gofmt -l .)
    if ($unformatted.Count -gt 0) {
        throw "gofmt failed:`n$($unformatted -join "`n")"
    }
    go mod tidy -diff
    if ($LASTEXITCODE -ne 0) { throw 'go mod tidy -diff failed' }
    go generate ./...
    if ($LASTEXITCODE -ne 0) { throw 'go generate failed' }
    git diff --exit-code -- api internal/transport/http/api
    if ($LASTEXITCODE -ne 0) { throw 'generated files are not clean' }
    go test ./... -count=1
    if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
    go test -race ./... -count=1
    if ($LASTEXITCODE -ne 0) { throw 'go test -race failed' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
    $previousCGO = $env:CGO_ENABLED
    try {
        $env:CGO_ENABLED = '0'
        go build -trimpath -buildvcs=false ./...
        if ($LASTEXITCODE -ne 0) { throw 'CGO-free build failed' }
    } finally {
        $env:CGO_ENABLED = $previousCGO
    }
    & (Join-Path $PSScriptRoot 'Verify-Artifacts.ps1')
} finally {
    Pop-Location
}
