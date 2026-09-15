$ErrorActionPreference = 'Stop'
$gradle = if ($env:GRADLE_CMD) { $env:GRADLE_CMD } else { 'gradle' }
& $gradle --no-daemon --console=plain clean lintRelease assembleRelease
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
