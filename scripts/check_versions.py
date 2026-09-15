from pathlib import Path
import json, re, sys

root = Path(__file__).resolve().parents[1]
version = (root / 'VERSION').read_text(encoding='utf-8').strip()
checks = {
    'apps/windows/portable/main.go': rf'appVersion\s*=\s*"{re.escape(version)}"',
    'apps/windows/setup/main.go': rf'appVersion\s*=\s*"{re.escape(version)}"',
    'apps/windows/build-release.ps1': rf'\$Version\s*=\s*"{re.escape(version)}"',
    'apps/android/app/build.gradle.kts': rf'versionName\s*=\s*"{re.escape(version)}"',
    'apps/android/app/src/main/java/net/radiobalkan/app/AppInfo.java': rf'VERSION\s*=\s*"{re.escape(version)}"',
    'README.md': rf'version {re.escape(version)}',
    'assets/badges/version.svg': rf'version: {re.escape(version)}',
}
errors=[]
for rel, pattern in checks.items():
    text=(root/rel).read_text(encoding='utf-8')
    if not re.search(pattern,text): errors.append(f'{rel}: version mismatch')
ext=json.loads((root/'extensions/version.json').read_text(encoding='utf-8'))
if ext.get('version') != version: errors.append('extensions/version.json: version mismatch')
for name in ('chrome','edge','opera','firefox'):
    data=json.loads((root/f'extensions/manifests/{name}.json').read_text(encoding='utf-8'))
    if data.get('name') != 'Radio Balkan': errors.append(f'{name}: brand name changed')
if errors:
    print('\n'.join(errors), file=sys.stderr); sys.exit(1)
print(f'Radio Balkan version sync OK: {version}')
