# Security Policy

## Supported Versions

Security fixes are applied to the latest release branch (`main`) and latest tagged release.

## Reporting a Vulnerability

Please do **not** open a public issue for sensitive vulnerabilities.

Report privately via:

- GitHub Security Advisories (preferred)
- or email: `abramovplay@gmail.com`

Include:

- affected version/tag
- reproduction steps
- impact assessment
- optional mitigation ideas

You can expect:

- acknowledgment within 72 hours
- triage and severity classification
- coordinated disclosure after a fix is available

## Scope Notes

Primary focus areas:

- unsafe file operations
- path traversal / unintended file access
- updater flow integrity
- privilege boundary issues on Windows

Out of scope:

- local-only issues requiring full trusted-machine access without privilege escalation
- non-security UX defects

