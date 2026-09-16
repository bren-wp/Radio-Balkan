param(
  [Parameter(Mandatory=$true)][string]$Exe,
  [Parameter(Mandatory=$true)][string]$Output,
  [ValidateSet('Auto','PrintWindow','ScreenCopy')][string]$Method = 'Auto'
)
$ErrorActionPreference = 'Stop'

$resolvedExe = (Resolve-Path -LiteralPath $Exe).Path
$outputPath = if ([IO.Path]::IsPathRooted($Output)) { $Output } else { Join-Path (Get-Location).Path $Output }
$outputPath = [IO.Path]::GetFullPath($outputPath)

if ($env:RADIO_BALKAN_CAPTURE_CHILD -ne '1') {
  $scriptPath = $MyInvocation.MyCommand.Path
  $shell = (Get-Process -Id $PID).Path
  $methods = if ($Method -eq 'Auto') { @('PrintWindow', 'ScreenCopy') } else { @($Method) }
  $failures = [System.Collections.Generic.List[string]]::new()

  foreach ($captureMethod in $methods) {
    $stamp = [Guid]::NewGuid().ToString('N')
    $stdout = Join-Path ([IO.Path]::GetTempPath()) "radio-balkan-capture-$stamp.out.log"
    $stderr = Join-Path ([IO.Path]::GetTempPath()) "radio-balkan-capture-$stamp.err.log"
    $attemptOutput = Join-Path ([IO.Path]::GetTempPath()) "radio-balkan-capture-$stamp.png"
    $child = $null
    $env:RADIO_BALKAN_CAPTURE_CHILD = '1'
    try {
      $arguments = @(
        '-NoLogo',
        '-NoProfile',
        '-NonInteractive',
        '-File', "`"$scriptPath`"",
        '-Exe', "`"$resolvedExe`"",
        '-Output', "`"$attemptOutput`"",
        '-Method', $captureMethod
      )
      $child = Start-Process -FilePath $shell -ArgumentList $arguments -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    } finally {
      Remove-Item Env:RADIO_BALKAN_CAPTURE_CHILD -ErrorAction SilentlyContinue
    }

    try {
      if (-not $child.WaitForExit(22000)) {
        & taskkill.exe /PID $child.Id /T /F | Out-Null
        throw "$captureMethod capture timed out after 22 seconds."
      }
      if (Test-Path $stdout) { Get-Content $stdout | Write-Host }
      if ($child.ExitCode -ne 0) {
        $details = if (Test-Path $stderr) { (Get-Content $stderr -Raw).Trim() } else { '' }
        throw "$captureMethod capture failed with exit code $($child.ExitCode). $details"
      }
      if (-not (Test-Path -LiteralPath $attemptOutput)) { throw "$captureMethod did not create a screenshot." }
      if ((Get-Item -LiteralPath $attemptOutput).Length -lt 1024) { throw "$captureMethod produced an unexpectedly small screenshot." }

      $dir = Split-Path -Parent $outputPath
      New-Item -ItemType Directory -Force -Path $dir | Out-Null
      Move-Item -LiteralPath $attemptOutput -Destination $outputPath -Force
      Write-Host "Windows UI captured with $captureMethod."
      return
    } catch {
      $failures.Add("${captureMethod}: $($_.Exception.Message)")
      Write-Warning "Windows UI capture attempt with $captureMethod failed: $($_.Exception.Message)"
    } finally {
      if ($child -and -not $child.HasExited) { & taskkill.exe /PID $child.Id /T /F | Out-Null }
      Remove-Item $stdout,$stderr,$attemptOutput -Force -ErrorAction SilentlyContinue
    }
  }

  throw "Windows UI capture failed for all bounded methods. $($failures -join ' | ')"
}

Add-Type -AssemblyName System.Drawing
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class WindowCaptureNative {
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hwnd, IntPtr hdcBlt, uint nFlags);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
}
'@

$p = Start-Process -FilePath $resolvedExe -PassThru
try {
  for ($i = 0; $i -lt 40 -and $p.MainWindowHandle -eq 0; $i++) {
    Start-Sleep -Milliseconds 250
    $p.Refresh()
  }
  if ($p.MainWindowHandle -eq 0) { throw 'Radio Balkan window was not created.' }
  [WindowCaptureNative]::ShowWindow($p.MainWindowHandle, 5) | Out-Null
  [WindowCaptureNative]::SetForegroundWindow($p.MainWindowHandle) | Out-Null
  Start-Sleep -Seconds 2
  $r = New-Object WindowCaptureNative+RECT
  if (-not [WindowCaptureNative]::GetWindowRect($p.MainWindowHandle, [ref]$r)) { throw 'GetWindowRect failed.' }
  $w = [Math]::Max(1, $r.Right - $r.Left)
  $h = [Math]::Max(1, $r.Bottom - $r.Top)
  $bmp = New-Object Drawing.Bitmap $w,$h
  try {
    $gfx = [Drawing.Graphics]::FromImage($bmp)
    try {
      if ($Method -eq 'ScreenCopy') {
        $size = New-Object Drawing.Size $w,$h
        $gfx.CopyFromScreen($r.Left, $r.Top, 0, 0, $size, [Drawing.CopyPixelOperation]::SourceCopy)
      } else {
        $hdc = $gfx.GetHdc()
        try {
          if (-not [WindowCaptureNative]::PrintWindow($p.MainWindowHandle, $hdc, 2)) {
            throw 'PrintWindow failed.'
          }
        } finally {
          $gfx.ReleaseHdc($hdc)
        }
      }
    } finally {
      $gfx.Dispose()
    }
    $dir = Split-Path -Parent $outputPath
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $bmp.Save($outputPath, [Drawing.Imaging.ImageFormat]::Png)
  } finally {
    if ($bmp) { $bmp.Dispose() }
  }
} finally {
  if ($p -and -not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
