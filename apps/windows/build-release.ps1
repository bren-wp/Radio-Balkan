param(
    [string]$Version = "0.0.26",
    [string]$Output = (Join-Path $PSScriptRoot "dist")
)

$ErrorActionPreference = "Stop"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go nije pronađen u PATH-u. Instaliraj aktualni Go SDK pa ponovno pokreni skriptu."
}

function Test-GoFormatting([string]$Path) {
    $resolved = (Resolve-Path $Path).Path
    $original = [IO.File]::ReadAllText($resolved).Replace("`r`n", "`n")
    $temp = [IO.Path]::GetTempFileName()
    try {
        [IO.File]::WriteAllText($temp, $original, [Text.UTF8Encoding]::new($false))
        & gofmt -w $temp
        if ($LASTEXITCODE -ne 0) {
            throw "gofmt provjera nije uspjela za $Path"
        }
        $formatted = [IO.File]::ReadAllText($temp).Replace("`r`n", "`n")
        if ($original -cne $formatted) {
            throw "$Path nije gofmt formatiran. Pokreni gofmt prije produkcijskog builda."
        }
    } finally {
        Remove-Item -Force -ErrorAction SilentlyContinue $temp
    }
}

New-Item -ItemType Directory -Force -Path $Output | Out-Null
$portable = Join-Path $Output "RadioBalkan-Portable-v$Version.exe"
$setup = Join-Path $Output "RadioBalkan-Setup-v$Version.exe"
$embeddedPortable = Join-Path $PSScriptRoot "setup\RadioBalkan-Portable.exe"

Push-Location (Join-Path $PSScriptRoot "portable")
try {
    Test-GoFormatting "main.go"
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "Windows Portable go vet nije uspio." }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "Windows Portable go test nije uspio." }
    go build -trimpath -buildvcs=false -ldflags "-s -w -H=windowsgui -X main.appVersion=$Version" -o $portable .
    if ($LASTEXITCODE -ne 0) { throw "Windows Portable go build nije uspio." }
} finally {
    Pop-Location
}

Copy-Item -Force $portable $embeddedPortable

Push-Location (Join-Path $PSScriptRoot "setup")
try {
    Test-GoFormatting "main.go"
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "Windows Setup go vet nije uspio." }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "Windows Setup go test nije uspio." }
    go build -trimpath -buildvcs=false -ldflags "-s -w -H=windowsgui -X main.appVersion=$Version" -o $setup .
    if ($LASTEXITCODE -ne 0) { throw "Windows Setup go build nije uspio." }
} finally {
    Pop-Location
    Remove-Item -Force -ErrorAction SilentlyContinue $embeddedPortable
}

$hashes = @(
    Get-FileHash -Algorithm SHA256 $portable
    Get-FileHash -Algorithm SHA256 $setup
)
$hashes | Format-Table -AutoSize
$hashes | ForEach-Object { "$($_.Hash.ToLower())  $([IO.Path]::GetFileName($_.Path))" } |
    Set-Content -Encoding ASCII (Join-Path $Output "RadioBalkan-v$Version-SHA256.txt")

Write-Host "Build završen: $Output"
