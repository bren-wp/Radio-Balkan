param(
  [Parameter(Mandatory=$true)][string[]]$Path,
  [switch]$Quiet
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

function Test-ScreenshotVisualContent {
  param([Parameter(Mandatory=$true)][string]$ImagePath)

  $resolved = (Resolve-Path -LiteralPath $ImagePath).Path
  $item = Get-Item -LiteralPath $resolved
  if ($item.Length -lt 1024) {
    throw "Screenshot is unexpectedly small: $resolved ($($item.Length) bytes)"
  }

  $bitmap = [Drawing.Bitmap]::FromFile($resolved)
  try {
    if ($bitmap.Width -lt 320 -or $bitmap.Height -lt 240) {
      throw "Screenshot dimensions are unexpectedly small: $resolved ($($bitmap.Width)x$($bitmap.Height))"
    }

    $stepX = [Math]::Max(1, [int][Math]::Floor($bitmap.Width / 48.0))
    $stepY = [Math]::Max(1, [int][Math]::Floor($bitmap.Height / 36.0))
    $colors = [System.Collections.Generic.HashSet[int]]::new()
    $minRgb = 765
    $maxRgb = 0
    $samples = 0
    $visibleSamples = 0

    for ($y = [int]($stepY / 2); $y -lt $bitmap.Height; $y += $stepY) {
      for ($x = [int]($stepX / 2); $x -lt $bitmap.Width; $x += $stepX) {
        $pixel = $bitmap.GetPixel($x, $y)
        [void]$colors.Add($pixel.ToArgb())
        $rgb = [int]$pixel.R + [int]$pixel.G + [int]$pixel.B
        if ($rgb -lt $minRgb) { $minRgb = $rgb }
        if ($rgb -gt $maxRgb) { $maxRgb = $rgb }
        if ($rgb -gt 36) { $visibleSamples++ }
        $samples++
      }
    }

    $range = $maxRgb - $minRgb
    $visibleRatio = if ($samples -gt 0) { $visibleSamples / [double]$samples } else { 0.0 }

    # Radio Balkan uses a dark UI, so absolute brightness is intentionally not
    # required. A real UI still contains multiple panel/text/accent colors and
    # measurable luminance variation. Uniform/near-black captures must fail.
    if ($colors.Count -lt 6 -or $range -lt 45 -or $visibleRatio -lt 0.02) {
      $detail = "Screenshot has insufficient visual content: {0} colors={1} rgbRange={2} visibleRatio={3:P1}" -f $resolved, $colors.Count, $range, $visibleRatio
      throw $detail
    }

    if (-not $Quiet) {
      Write-Host ("Screenshot quality OK: {0} ({1}x{2}, sampledColors={3}, rgbRange={4})" -f $resolved, $bitmap.Width, $bitmap.Height, $colors.Count, $range)
    }
  } finally {
    $bitmap.Dispose()
  }
}

foreach ($candidate in $Path) {
  Test-ScreenshotVisualContent -ImagePath $candidate
}
