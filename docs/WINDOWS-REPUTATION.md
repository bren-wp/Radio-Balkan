# Windows security reputation

Radio Balkan is distributed as native Windows executables. Antivirus and SmartScreen products may use static ML, behavioral heuristics, publisher reputation, and file-hash reputation.

## Production requirements

Public Windows releases must:

- be built from the exact CI-verified `main` commit;
- keep Go symbols/build metadata intact (do not reintroduce `-s -w`);
- be signed with a trusted Authenticode Code Signing certificate;
- timestamp the signature;
- verify the signature after signing and before upload;
- publish SHA-256 checksums;
- never modify an EXE after it has been signed.

The Publish workflow expects these GitHub Actions secrets:

- `RADIO_BALKAN_WINDOWS_PFX_B64` — Base64-encoded PFX/PKCS#12 certificate;
- `RADIO_BALKAN_WINDOWS_PFX_PASSWORD` — password for that PFX.

The certificate must contain the Code Signing EKU `1.3.6.1.5.5.7.3.3`. Public Windows publishing intentionally fails when the signing identity is missing or invalid.

## Current heuristic risk to remove

The Windows Portable player still contains a legacy WPF audio backend that starts hidden `powershell.exe`, loads WPF assemblies, and compiles a C# media host through `Add-Type`.

This code is legitimate, but it resembles behaviors commonly used by loaders and droppers and therefore increases false-positive risk in static and behavioral antivirus models.

Do not attempt to hide, obfuscate, rename, encode, or otherwise disguise this behavior. The correct remediation is to replace the legacy helper with an in-process Windows media backend (Media Foundation/Media Session or another transparent native playback path), while retaining the existing playback lifecycle and regression tests.

Until that migration is complete:

- do not claim that signing alone guarantees zero detections;
- do not weaken antivirus or SmartScreen checks for users;
- do not add exclusions or Defender-bypass instructions to the product;
- do not use packers such as UPX for public binaries.

## False-positive handling

If a clean, signed release is incorrectly detected:

1. Record the exact release tag, commit SHA, file SHA-256, signing publisher, and detector name.
2. Verify the GitHub Release asset hash matches the published SHA-256 manifest.
3. Submit the exact signed binary to the detecting vendor as a false positive.
4. For Microsoft Defender detections, use Microsoft's malware-analysis submission portal as a software developer.
5. For VirusTotal detections, contact the individual antivirus vendor; VirusTotal aggregates vendor verdicts and does not override them.
6. Keep the same trusted signing identity across releases so publisher reputation can accumulate.

## Release gate

A release is not considered Windows-production-ready unless:

- Portable and Setup both have `Valid` Authenticode signatures;
- CI and runtime smoke tests are green;
- checksums are generated and verified;
- no post-signing mutation occurs;
- the release commit matches the tag target exactly.
