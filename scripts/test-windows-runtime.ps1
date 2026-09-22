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
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);
  [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);

  public static int CountVisibleTopLevelWindowsForProcess(int processId) {
    int count = 0;
    EnumWindows((hWnd, lParam) => {
      uint pid;
      GetWindowThreadProcessId(hWnd, out pid);
      if (pid == (uint)processId && IsWindowVisible(hWnd)) {
        count++;
      }
      return true;
    }, IntPtr.Zero);
    return count;
  }
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
    $visibleWindows = [RadioBalkanSmokeNative]::CountVisibleTopLevelWindowsForProcess($p.Id)
    if ($visibleWindows -ne 1) {
      $details = Get-CrashDetails
      throw ("Radio Balkan opened {0} visible top-level windows during normal navigation; expected exactly one main window.`n{1}" -f $visibleWindows, $details)
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
  $inputMarker = "[runtime-test] input-smoke-ok token=$runtimeToken"
  $audioMarker = "[runtime-test] audio-smoke-ok token=$runtimeToken"
  $noDeviceMarker = "[runtime-test] audio-smoke-no-device token=$runtimeToken"
  $switchMarker = "[runtime-test] production-audio-switch-ok token=$runtimeToken"
  $switchNoDeviceMarker = "[runtime-test] production-audio-switch-no-device token=$runtimeToken"
  $inputOk = $lines | Where-Object { $_.Contains($inputMarker) } | Select-Object -First 1
  $audioOpened = $lines | Where-Object { $_.Contains($audioMarker) } | Select-Object -First 1
  $noDevice = $lines | Where-Object { $_.Contains($noDeviceMarker) } | Select-Object -First 1
  $switchOpened = $lines | Where-Object { $_.Contains($switchMarker) } | Select-Object -First 1
  $switchNoDevice = $lines | Where-Object { $_.Contains($switchNoDeviceMarker) } | Select-Object -First 1
  if (-not $inputOk) {
    $details = Get-CrashDetails
    throw ("Radio Balkan did not complete the real mouse-click responsiveness smoke: sidebar/header Countries+Genres, resize/repaint hit regions, native search focus, favorite, station details/back, rendered Play, Stop, single-window navigation, full-catalog repaint, supplemental filters, and volume/navigation while the audio backend lock was held.`n{0}" -f $details)
  }
  if (-not $switchOpened -and -not $switchNoDevice) {
    $details = Get-CrashDetails
    throw ("Radio Balkan production playback did not complete the A-to-B station-switch smoke or return the recognized headless audio-device result.`n{0}" -f $details)
  }
  if (-not $audioOpened -and -not $noDevice) {
    $details = Get-CrashDetails
    throw ("Radio Balkan audio engine neither opened HTTP media nor returned the recognized headless-runner audio-device result.`n{0}" -f $details)
  }
  if ($noDevice -or $switchNoDevice) {
    Write-Host ("Radio Balkan runtime smoke OK: click/navigation and A-to-B production switch paths executed; Windows returned the expected no-audio-device result on this headless runner; shutdown remained healthy.")
  } else {
    Write-Host ("Radio Balkan runtime smoke OK: real click/navigation responsiveness passed, production audio switched A-to-B in one app process, UI stayed alive for {0} seconds, HTTP audio opened successfully, and WM_CLOSE shutdown completed." -f $SoakSeconds)
  }
} finally {
  if ($p -and -not $p.HasExited) {
    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  }
  $env:RADIO_BALKAN_RUNTIME_TEST = $previousRuntimeTest
  $env:RADIO_BALKAN_RUNTIME_TOKEN = $previousRuntimeToken
}
