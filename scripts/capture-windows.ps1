param(
  [Parameter(Mandatory=$true)][string]$Exe,
  [Parameter(Mandatory=$true)][string]$Output
)
$ErrorActionPreference = 'Stop'

$resolvedExe = (Resolve-Path -LiteralPath $Exe).Path
$outputPath = if ([IO.Path]::IsPathRooted($Output)) { $Output } else { Join-Path (Get-Location).Path $Output }
$outputPath = [IO.Path]::GetFullPath($outputPath)

if ($env:RADIO_BALKAN_CAPTURE_CHILD -ne '1') {
  $scriptPath = $MyInvocation.MyCommand.Path
  $shell = (Get-Process -Id $PID).Path
  $stamp = [Guid]::NewGuid().ToString('N')
  $stdout = Join-Path ([IO.Path]::GetTempPath()) "radio-balkan-capture-$stamp.out.log"
  $stderr = Join-Path ([IO.Path]::GetTempPath()) "radio-balkan-capture-$stamp.err.log"
  $child = $null
  $env:RADIO_BALKAN_CAPTURE_CHILD = '1'
  try {
    $arguments = @(
      '-NoLogo',
      '-NoProfile',
      '-NonInteractive',
      '-File', "`"$scriptPath`"",
      '-Exe', "`"$resolvedExe`"",
      '-Output', "`"$outputPath`""
    )
    $child = Start-Process -FilePath $shell -ArgumentList $arguments -PassThru -WindowStyle Hidden `
      -RedirectStandardOutput $stdout -RedirectStandardError $stderr
  } finally {
    Remove-Item Env:RADIO_BALKAN_CAPTURE_CHILD -ErrorAction SilentlyContinue
  }

  try {
    if (-not $child.WaitForExit(30000)) {
      & taskkill.exe /PID $child.Id /T /F | Out-Null
      throw 'Windows UI capture timed out after 30 seconds.'
    }
    if (Test-Path $stdout) { Get-Content $stdout | Write-Host }
    if ($child.ExitCode -ne 0) {
      $details = if (Test-Path $stderr) { (Get-Content $stderr -Raw).Trim() } else { '' }
      throw "Windows UI capture failed with exit code $($child.ExitCode). $details"
    }
    if (-not (Test-Path -LiteralPath $outputPath)) { throw 'Windows screenshot was not created.' }
    if ((Get-Item -LiteralPath $outputPath).Length -lt 1024) { throw 'Windows screenshot is unexpectedly small.' }
  } finally {
    if ($child -and -not $child.HasExited) { & taskkill.exe /PID $child.Id /T /F | Out-Null }
    Remove-Item $stdout,$stderr -Force -ErrorAction SilentlyContinue
  }
  return
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
  Start-Sleep -Seconds 3
  $r = New-Object WindowCaptureNative+RECT
  if (-not [WindowCaptureNative]::GetWindowRect($p.MainWindowHandle, [ref]$r)) { throw 'GetWindowRect failed.' }
  $w = [Math]::Max(1, $r.Right - $r.Left)
  $h = [Math]::Max(1, $r.Bottom - $r.Top)
  $bmp = New-Object Drawing.Bitmap $w,$h
  try {
    $gfx = [Drawing.Graphics]::FromImage($bmp)
    try {
      $hdc = $gfx.GetHdc()
      try {
        if (-not [WindowCaptureNative]::PrintWindow($p.MainWindowHandle, $hdc, 2)) {
          throw 'PrintWindow failed.'
        }
      } finally {
        $gfx.ReleaseHdc($hdc)
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
