param(
    [string]$Version = "0.0.44",
    [string]$Output = (Join-Path $PSScriptRoot "dist"),
    [string]$SigningThumbprint = $env:RADIO_BALKAN_SIGNING_THUMBPRINT,
    [string]$TimestampUrl = "http://timestamp.digicert.com",
    [switch]$RequireSignature
)

$ErrorActionPreference = "Stop"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go nije pronađen u PATH-u. Instaliraj aktualni Go SDK pa ponovno pokreni skriptu."
}

function Invoke-CodeSign([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($SigningThumbprint)) {
        if ($RequireSignature) {
            throw "Release signature is required, but RADIO_BALKAN_SIGNING_THUMBPRINT is not configured."
        }
        Write-Host "Building unsigned Windows artifact."
        return
    }

    $signTool = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if (-not $signTool) {
        throw "signtool.exe was not found in PATH."
    }

    & $signTool.Source sign /sha1 $SigningThumbprint /fd SHA256 /tr $TimestampUrl /td SHA256 /v $Path
    if ($LASTEXITCODE -ne 0) {
        throw "Authenticode signing failed for $Path"
    }

    $signature = Get-AuthenticodeSignature -FilePath $Path
    if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid) {
        throw "Authenticode signature is not valid for $Path ($($signature.Status))"
    }
}

function Test-GoFormatting([string]$Path) {
    $resolved = (Resolve-Path $Path).Path
    $original = [IO.File]::ReadAllText($resolved).Replace("`r`n", "`n")
    $temp = [IO.Path]::GetTempFileName()
    try {
        [IO.File]::WriteAllText($temp, $original, [Text.UTF8Encoding]::new($false))
        $formatDiff = & gofmt -d $temp 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "gofmt provjera nije uspjela za $Path"
        }
        & gofmt -w $temp
        if ($LASTEXITCODE -ne 0) {
            throw "gofmt provjera nije uspjela za $Path"
        }
        $formatted = [IO.File]::ReadAllText($temp).Replace("`r`n", "`n")
        if ($original -cne $formatted) {
            if ($formatDiff) { $formatDiff | Write-Host }
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
    go build -trimpath -buildvcs=false -ldflags "-H=windowsgui -X main.appVersion=$Version" -o $portable .
    if ($LASTEXITCODE -ne 0) { throw "Windows Portable go build nije uspio." }
} finally {
    Pop-Location
}

Invoke-CodeSign $portable
Copy-Item -Force $portable $embeddedPortable

Push-Location (Join-Path $PSScriptRoot "setup")
try {
    Test-GoFormatting "main.go"
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "Windows Setup go vet nije uspio." }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "Windows Setup go test nije uspio." }
    go build -trimpath -buildvcs=false -ldflags "-H=windowsgui -X main.appVersion=$Version" -o $setup .
    if ($LASTEXITCODE -ne 0) { throw "Windows Setup go build nije uspio." }
} finally {
    Pop-Location
    Remove-Item -Force -ErrorAction SilentlyContinue $embeddedPortable
}

Invoke-CodeSign $setup

$hashes = @(
    Get-FileHash -Algorithm SHA256 $portable
    Get-FileHash -Algorithm SHA256 $setup
)
$hashes | Format-Table -AutoSize
$hashes | ForEach-Object { "$($_.Hash.ToLower())  $([IO.Path]::GetFileName($_.Path))" } |
    Set-Content -Encoding ASCII (Join-Path $Output "RadioBalkan-v$Version-SHA256.txt")

Write-Host "Build završen: $Output"
