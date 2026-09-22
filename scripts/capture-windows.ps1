param(
  [Parameter(Mandatory=$true)][string]$Exe,
  [Parameter(Mandatory=$true)][string]$Output
)
$ErrorActionPreference = 'Stop'

$resolvedExe = (Resolve-Path -LiteralPath $Exe).Path
$outputPath = if ([IO.Path]::IsPathRooted($Output)) { $Output } else { Join-Path (Get-Location).Path $Output }
$outputPath = [IO.Path]::GetFullPath($outputPath)
$qualityScript = Join-Path $PSScriptRoot 'validate-screenshot.ps1'
if (-not (Test-Path -LiteralPath $qualityScript)) { throw 'Screenshot quality validator was not found.' }

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
    $child = Start-Process -FilePath $shell -ArgumentList $arguments -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
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
    & $qualityScript -Path @($outputPath) -Quiet
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
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool UpdateWindow(IntPtr hWnd);
}
'@

function Save-PrintWindowCapture {
  param(
    [Parameter(Mandatory=$true)][IntPtr]$Hwnd,
    [Parameter(Mandatory=$true)][int]$Width,
    [Parameter(Mandatory=$true)][int]$Height,
    [Parameter(Mandatory=$true)][uint32]$Flags,
    [Parameter(Mandatory=$true)][string]$Candidate
  )
  $bitmap = [Drawing.Bitmap]::new($Width, $Height)
  try {
    $graphics = [Drawing.Graphics]::FromImage($bitmap)
    try {
      $hdc = $graphics.GetHdc()
      try {
        $ok = [WindowCaptureNative]::PrintWindow($Hwnd, $hdc, $Flags)
      } finally {
        $graphics.ReleaseHdc($hdc)
      }
    } finally {
      $graphics.Dispose()
    }
    if (-not $ok) { return $false }
    $bitmap.Save($Candidate, [Drawing.Imaging.ImageFormat]::Png)
    return $true
  } finally {
    $bitmap.Dispose()
  }
}

function Save-ScreenCopyCapture {
  param(
    [Parameter(Mandatory=$true)][int]$Left,
    [Parameter(Mandatory=$true)][int]$Top,
    [Parameter(Mandatory=$true)][int]$Width,
    [Parameter(Mandatory=$true)][int]$Height,
    [Parameter(Mandatory=$true)][string]$Candidate
  )
  $bitmap = [Drawing.Bitmap]::new($Width, $Height)
  try {
    $graphics = [Drawing.Graphics]::FromImage($bitmap)
    try {
      $size = [Drawing.Size]::new($Width, $Height)
      $graphics.CopyFromScreen($Left, $Top, 0, 0, $size, [Drawing.CopyPixelOperation]::SourceCopy)
    } finally {
      $graphics.Dispose()
    }
    $bitmap.Save($Candidate, [Drawing.Imaging.ImageFormat]::Png)
    return $true
  } finally {
    $bitmap.Dispose()
  }
}

function Test-CaptureCandidate {
  param([Parameter(Mandatory=$true)][string]$Candidate, [Parameter(Mandatory=$true)][string]$Method)
  try {
    & $qualityScript -Path @($Candidate) -Quiet
    Write-Host "Windows screenshot capture accepted: $Method"
    return $true
  } catch {
    Write-Warning "Windows screenshot capture rejected ($Method): $($_.Exception.Message)"
    return $false
  }
}

$p = Start-Process -FilePath $resolvedExe -PassThru
try {
  for ($i = 0; $i -lt 40 -and $p.MainWindowHandle -eq 0; $i++) {
    Start-Sleep -Milliseconds 250
    $p.Refresh()
  }
  if ($p.MainWindowHandle -eq 0) { throw 'Radio Balkan window was not created.' }

  [WindowCaptureNative]::ShowWindow($p.MainWindowHandle, 9) | Out-Null
  [WindowCaptureNative]::SetForegroundWindow($p.MainWindowHandle) | Out-Null
  [WindowCaptureNative]::UpdateWindow($p.MainWindowHandle) | Out-Null
  Start-Sleep -Seconds 3

  $r = New-Object WindowCaptureNative+RECT
  if (-not [WindowCaptureNative]::GetWindowRect($p.MainWindowHandle, [ref]$r)) { throw 'GetWindowRect failed.' }
  $w = [Math]::Max(1, $r.Right - $r.Left)
  $h = [Math]::Max(1, $r.Bottom - $r.Top)
  $dir = Split-Path -Parent $outputPath
  New-Item -ItemType Directory -Force -Path $dir | Out-Null

  $captured = $false
  $attempts = @(
    @{ Name = 'PrintWindow'; Flags = [uint32]0 },
    @{ Name = 'PrintWindowFullContent'; Flags = [uint32]2 }
  )
  foreach ($attempt in $attempts) {
    $candidate = "$outputPath.$($attempt.Name).tmp.png"
    Remove-Item -LiteralPath $candidate -Force -ErrorAction SilentlyContinue
    try {
      if ((Save-PrintWindowCapture -Hwnd $p.MainWindowHandle -Width $w -Height $h -Flags $attempt.Flags -Candidate $candidate) -and (Test-CaptureCandidate -Candidate $candidate -Method $attempt.Name)) {
        Move-Item -LiteralPath $candidate -Destination $outputPath -Force
        $captured = $true
        break
      }
    } finally {
      Remove-Item -LiteralPath $candidate -Force -ErrorAction SilentlyContinue
    }
  }

  if (-not $captured) {
    [WindowCaptureNative]::SetForegroundWindow($p.MainWindowHandle) | Out-Null
    Start-Sleep -Milliseconds 750
    $candidate = "$outputPath.ScreenCopy.tmp.png"
    Remove-Item -LiteralPath $candidate -Force -ErrorAction SilentlyContinue
    try {
      if ((Save-ScreenCopyCapture -Left $r.Left -Top $r.Top -Width $w -Height $h -Candidate $candidate) -and (Test-CaptureCandidate -Candidate $candidate -Method 'ScreenCopy')) {
        Move-Item -LiteralPath $candidate -Destination $outputPath -Force
        $captured = $true
      }
    } finally {
      Remove-Item -LiteralPath $candidate -Force -ErrorAction SilentlyContinue
    }
  }

  if (-not $captured) {
    throw 'All Windows screenshot capture methods produced blank or invalid output.'
  }
} finally {
  if ($p -and -not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
