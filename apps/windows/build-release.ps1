param(
    [string]$Version = "0.0.6",
    [string]$Output = (Join-Path $PSScriptRoot "dist")
)

$ErrorActionPreference = "Stop"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go nije pronađen u PATH-u. Instaliraj aktualni Go SDK pa ponovno pokreni skriptu."
}

New-Item -ItemType Directory -Force -Path $Output | Out-Null
$portable = Join-Path $Output "RadioBalkan-Portable-v$Version.exe"
$setup = Join-Path $Output "RadioBalkan-Setup-v$Version.exe"

Push-Location (Join-Path $PSScriptRoot "portable")
try {
    gofmt -w main.go
    go vet ./...
    go test ./...
    go build -trimpath -buildvcs=false -ldflags "-s -w -H=windowsgui -X main.appVersion=$Version" -o $portable .
} finally { Pop-Location }

Copy-Item -Force $portable (Join-Path $PSScriptRoot "setup\RadioBalkan-Portable.exe")

Push-Location (Join-Path $PSScriptRoot "setup")
try {
    gofmt -w main.go
    go vet ./...
    go test ./...
    go build -trimpath -buildvcs=false -ldflags "-s -w -H=windowsgui -X main.appVersion=$Version" -o $setup .
} finally { Pop-Location }

$hashes = @(
    Get-FileHash -Algorithm SHA256 $portable
    Get-FileHash -Algorithm SHA256 $setup
)
$hashes | Format-Table -AutoSize
$hashes | ForEach-Object { "$($_.Hash.ToLower())  $([IO.Path]::GetFileName($_.Path))" } |
    Set-Content -Encoding ASCII (Join-Path $Output "RadioBalkan-v$Version-SHA256.txt")

Write-Host "Build završen: $Output"
