param(
  [Parameter(Mandatory=$true)][string]$Exe,
  [Parameter(Mandatory=$true)][string]$Output
)
$ErrorActionPreference = 'Stop'
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
$p = Start-Process -FilePath $Exe -PassThru
try {
  for ($i=0; $i -lt 40 -and $p.MainWindowHandle -eq 0; $i++) { Start-Sleep -Milliseconds 250; $p.Refresh() }
  if ($p.MainWindowHandle -eq 0) { throw 'Radio Balkan window was not created.' }
  [WindowCaptureNative]::ShowWindow($p.MainWindowHandle, 5) | Out-Null
  Start-Sleep -Seconds 3
  $r = New-Object WindowCaptureNative+RECT
  if (-not [WindowCaptureNative]::GetWindowRect($p.MainWindowHandle, [ref]$r)) { throw 'GetWindowRect failed.' }
  $w = [Math]::Max(1, $r.Right - $r.Left); $h = [Math]::Max(1, $r.Bottom - $r.Top)
  $bmp = New-Object Drawing.Bitmap $w,$h
  $gfx = [Drawing.Graphics]::FromImage($bmp)
  $hdc = $gfx.GetHdc()
  try { [WindowCaptureNative]::PrintWindow($p.MainWindowHandle, $hdc, 2) | Out-Null } finally { $gfx.ReleaseHdc($hdc); $gfx.Dispose() }
  $dir = Split-Path -Parent $Output; New-Item -ItemType Directory -Force -Path $dir | Out-Null
  $bmp.Save($Output, [Drawing.Imaging.ImageFormat]::Png); $bmp.Dispose()
} finally {
  if ($p -and -not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
