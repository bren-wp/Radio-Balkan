# Windows security reputation

Radio Balkan is distributed as native Windows executables. Antivirus and SmartScreen products may use static ML, behavioral heuristics, file-hash reputation, runtime behavior, embedded resources, process creation, and network activity.

## Production requirements

Public Windows releases must:

- be built from the exact CI-verified `main` commit;
- keep Go symbols/build metadata intact (do not reintroduce `-s -w`);
- pass Go tests, vet, Windows runtime smoke tests, click-routing coverage, and screenshot quality gates;
- publish SHA-256 checksums for every artifact;
- keep the GitHub tag target identical to the release commit;
- avoid packers, obfuscators, self-modifying code, and antivirus-bypass behavior.

Authenticode signing is optional and is not a Radio Balkan release requirement. The public GitHub Publish workflow intentionally supports unsigned Windows artifacts.

## Current heuristic risk to remove

The Windows Portable player still contains a legacy WPF audio backend that starts hidden `powershell.exe`, loads WPF assemblies, and compiles a C# media host through `Add-Type`.

This code is legitimate, but it resembles behaviors commonly used by loaders and droppers and therefore increases false-positive risk in static and behavioral antivirus models.

Do not attempt to hide, obfuscate, rename, encode, or otherwise disguise this behavior. The correct remediation is to replace the legacy helper with an in-process Windows media backend such as Media Foundation/MFPlay, while retaining the existing playback lifecycle and regression tests.

The Windows Setup also contains behaviors that can contribute to heuristic risk: embedding another executable, writing executable files, creating an uninstaller, using helper processes for shortcuts/registry/startup operations, and self-cleanup. Prefer direct Windows APIs where practical and keep every such operation explicit, deterministic, and covered by tests.

## False-positive handling

If a clean release is incorrectly detected:

1. Record the exact release tag, commit SHA, file SHA-256, and detector name.
2. Verify the GitHub Release asset hash matches the published SHA-256 manifest.
3. Reproduce the finding against the exact release artifact, not a locally rebuilt binary.
4. Submit that exact binary to the detecting vendor as a false positive.
5. For Microsoft Defender detections, use Microsoft's malware-analysis submission portal as a software developer.
6. For VirusTotal detections, contact the individual antivirus vendor; VirusTotal aggregates vendor verdicts and does not override them.

## Release gate

A release is considered Windows-production-ready when:

- CI and the real Windows runtime smoke tests are green;
- click-by-click navigation/playback coverage is green;
- screenshot quality validation is green;
- checksums are generated and verified;
- the repository remains clean after build;
- the release commit matches the tag target exactly.

Signing is not part of this gate.
