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
  if (-not (Test-Path -LiteralPath $logPath)) {
    return '<no app.log was produced>'
  }

  $lines = @(Get-Content -LiteralPath $logPath -ErrorAction SilentlyContinue)
  if ($lines.Count -eq 0) {
    return '<app.log was empty>'
  }

  $trace = @($lines | Where-Object { $_ -match '\[runtime-test\]' } | Select-Object -Last 24)
  $stackStart = -1
  for ($i = $lines.Count - 1; $i -ge 0; $i--) {
    if ($lines[$i] -match '\[runtime-test-stacks\]') {
      $stackStart = $i
      break
    }
  }

  $stack = @()
  if ($stackStart -ge 0) {
    $stackEnd = [Math]::Min($lines.Count - 1, $stackStart + 240)
    if ($stackEnd -eq $stackStart) {
      $stack = @($lines[$stackStart])
    } else {
      $stack = @($lines[$stackStart..$stackEnd])
    }
  } else {
    $stack = @($lines | Select-Object -Last 160)
  }

  $details = @('Recent runtime trace:')
  if ($trace.Count -gt 0) {
    $details += $trace
  } else {
    $details += '<no runtime trace markers>'
  }
  $details += ''
  $details += 'Latest stalled stack:'
  $details += $stack
  return ($details -join [Environment]::NewLine)
}

$previousRuntimeTest = $env:RADIO_BALKAN_RUNTIME_TEST
$previousRuntimeToken = $env:RADIO_BALKAN_RUNTIME_TOKEN
$runtimeToken = [Guid]::NewGuid().ToString('N')
$env:RADIO_BALKAN_RUNTIME_TEST = '1'
$env:RADIO_BALKAN_RUNTIME_TOKEN = $runtimeToken
$p = Start-Process -FilePath $resolvedExe -ArgumentList '--ci-runtime-smoke' -PassThru
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

  if (-not $p.WaitForExit(12000)) {
    $details = Get-CrashDetails
    throw ("Radio Balkan did not complete its CI self-close after the startup soak.`n{0}" -f $details)
  }
  if ($p.ExitCode -ne 0) {
    $details = Get-CrashDetails
    throw ("Radio Balkan returned non-zero exit code {0} after its CI self-close.`n{1}" -f $p.ExitCode, $details)
  }
  $lines = @()
  if (Test-Path -LiteralPath $logPath) {
    $lines = @(Get-Content -LiteralPath $logPath -ErrorAction SilentlyContinue)
  }
  $audioMarker = "[runtime-test] audio-smoke-ok token=$runtimeToken"
  $noDeviceMarker = "[runtime-test] audio-smoke-no-device token=$runtimeToken"
  $audioOpened = $lines | Where-Object { $_.Contains($audioMarker) } | Select-Object -First 1
  $noDevice = $lines | Where-Object { $_.Contains($noDeviceMarker) } | Select-Object -First 1
  if (-not $audioOpened -and -not $noDevice) {
    $details = Get-CrashDetails
    throw ("Radio Balkan audio engine neither opened HTTP media nor returned the recognized headless-runner audio-device result.`n{0}" -f $details)
  }
  if ($noDevice) {
    Write-Host ("Radio Balkan runtime smoke OK: media open reached Windows MediaPlayer and returned the expected no-audio-device result on this headless runner; UI and shutdown remained healthy.")
  } else {
    Write-Host ("Radio Balkan runtime smoke OK: UI stayed alive for {0} seconds, HTTP audio opened successfully, and WM_CLOSE shutdown completed." -f $SoakSeconds)
  }
} finally {
  if ($p -and -not $p.HasExited) {
    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  }
  $env:RADIO_BALKAN_RUNTIME_TEST = $previousRuntimeTest
  $env:RADIO_BALKAN_RUNTIME_TOKEN = $previousRuntimeToken
}
