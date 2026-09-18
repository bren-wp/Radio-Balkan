param(
  [Parameter(Mandatory=$true)][string]$Exe,
  [int]$SoakSeconds = 15
)
$ErrorActionPreference = 'Stop'
if ($SoakSeconds -lt 8 -or $SoakSeconds -gt 60) { throw 'SoakSeconds must be between 8 and 60.' }

$resolvedExe = (Resolve-Path -LiteralPath $Exe).Path
$logPath = Join-Path $env:LOCALAPPDATA 'RadioBalkan\logs\app.log'

Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class RadioBalkanSmokeNative {
  [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr hWnd);
}
'@

function Get-CrashDetails {
  if (Test-Path -LiteralPath $logPath) {
    return ((Get-Content -LiteralPath $logPath -Tail 120 -ErrorAction SilentlyContinue) -join [Environment]::NewLine)
  }
  return '<no app.log was produced>'
}

$previousRuntimeTest = $env:RADIO_BALKAN_RUNTIME_TEST
$env:RADIO_BALKAN_RUNTIME_TEST = '1'
$p = Start-Process -FilePath $resolvedExe -PassThru
try {
  $window = [IntPtr]::Zero
  for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 200
    $p.Refresh()
    if ($p.HasExited) {
      $details = Get-CrashDetails
      throw ("Radio Balkan exited during startup with code {0}.`n{1}" -f $p.ExitCode, $details)
    }
    if ($p.MainWindowHandle -ne 0) {
      $window = [IntPtr]$p.MainWindowHandle
      break
    }
  }
  if ($window -eq [IntPtr]::Zero) {
    $details = Get-CrashDetails
    throw ("Radio Balkan did not create its main window within 8 seconds.`n{0}" -f $details)
  }

  $deadline = [DateTime]::UtcNow.AddSeconds($SoakSeconds)
  while ([DateTime]::UtcNow -lt $deadline) {
    Start-Sleep -Milliseconds 500
    $p.Refresh()
    if ($p.HasExited) {
      $details = Get-CrashDetails
      throw ("Radio Balkan exited during the {0}-second startup soak with code {1}.`n{2}" -f $SoakSeconds, $p.ExitCode, $details)
    }
    if (-not [RadioBalkanSmokeNative]::IsWindow([IntPtr]$p.MainWindowHandle)) {
      $details = Get-CrashDetails
      throw ("Radio Balkan lost its main window during startup soak.`n{0}" -f $details)
    }
  }

  $p.Refresh()
  if (-not $p.CloseMainWindow()) {
    $details = Get-CrashDetails
    throw ("Radio Balkan main window rejected the close request.`n{0}" -f $details)
  }
  if (-not $p.WaitForExit(8000)) {
    $details = Get-CrashDetails
    throw ("Radio Balkan did not exit cleanly after WM_CLOSE.`n{0}" -f $details)
  }
  if ($p.ExitCode -ne 0) {
    $details = Get-CrashDetails
    throw ("Radio Balkan returned non-zero exit code {0} after a clean close request.`n{1}" -f $p.ExitCode, $details)
  }
  Write-Host ("Radio Balkan runtime smoke OK: main window stayed alive for {0} seconds and closed cleanly." -f $SoakSeconds)
} finally {
  if ($p -and -not $p.HasExited) {
    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  }
  $env:RADIO_BALKAN_RUNTIME_TEST = $previousRuntimeTest
}
