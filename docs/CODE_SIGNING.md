# Code Signing Plan

Current status: binaries are unsigned.

## Why signing matters

- Reduces SmartScreen/AV trust friction
- Improves enterprise deployment acceptance
- Gives users stronger integrity signals

## Planned flow

1. Obtain EV/OV code-signing certificate.
2. Sign `icicle.exe` and `icicle-desktop.exe` in release pipeline.
3. Publish SHA256 checksums for release archives.
4. Verify signatures in QA before publishing tags.

## Interim safeguards

- Reproducible build commands in docs
- SHA256 checksum artifact from `scripts/release.ps1`
- Open-source reviewable codebase and CI
