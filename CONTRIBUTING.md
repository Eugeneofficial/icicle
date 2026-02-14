# Contributing to icicle

Thanks for contributing.

## Workflow

1. Fork the repository.
2. Create a feature branch from `main`.
3. Make focused changes with clear commit messages.
4. Run checks locally:
   - `go test ./...`
   - `go build -o icicle.exe ./cmd/icicle`
   - `go build -tags "wails,production" -o icicle-desktop.exe ./cmd/icicle-wails`
5. Open a Pull Request with:
   - problem statement
   - what changed
   - before/after behavior
   - test notes

## Standards

- Keep changes small and reviewable.
- Preserve Windows-first behavior.
- Avoid breaking CLI compatibility.
- Do not commit generated runtime data (`portable-data/`, local binaries).
- Keep RU/EN text consistent when changing UI labels.

## Bug Reports

Include:

- exact command or UI action
- expected vs actual behavior
- full error output / stack trace
- OS version and Go version

## Feature Requests

Provide:

- use case
- proposed UX/API
- tradeoffs and constraints

## Code Style

- Use `gofmt`.
- Prefer clear, explicit naming.
- Add comments only where logic is non-obvious.
- Keep performance-sensitive paths allocation-light.

