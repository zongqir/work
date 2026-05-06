# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go module (`module notes`) with most implementation under `code/aggregate_registry_demo/`. Root-level `.md` files are design and requirements notes. Inside the demo module, packages are organized by responsibility: `contract/`, `config/`, `dispatch/`, `delivery/`, `render/`, `scheduler/`, `publisher/`, `preview/`, `bootstrap/`, and `handlers/<message_type>/`. Static templates live in `templates/`, and sample inputs/outputs are stored as JSON alongside the demo code.

## Build, Test, and Development Commands
- `go test ./...` runs the full test suite from the repository root.
- `go build ./...` verifies all packages compile.
- `cd code/aggregate_registry_demo; go test ./preview` runs the preview-focused package tests.

## Coding Style & Naming Conventions
Use standard Go formatting (`gofmt`) and idiomatic Go naming: short package names, exported identifiers in `CamelCase`, unexported identifiers in `camelCase`, and tests in `*_test.go`. Keep package boundaries narrow and prefer one `handler.go` per `handlers/<message_type>/` directory. JSON/template fixture names should stay descriptive and aligned with the message type, such as `xdr_risk_digest_default.body.tmpl`.

## Testing Guidelines
Tests use Go’s built-in `testing` package. Name tests `TestXxx` and keep package-local fakes/stubs close to the package under test. Prefer table-driven tests for config, rendering, and dispatch logic. When changing behavior, run `go test ./...` before submitting.

## Commit & Pull Request Guidelines
Recent commits in this repository use short, minimal subjects, so keep commit messages concise and imperative when possible. PRs should include a brief summary, the affected package(s), and validation notes (`go test ./...`, `go build ./...`, or package-specific tests). Add sample output or screenshots only when the change affects rendered templates or generated artifacts.

## Agent Notes
Treat `code/aggregate_registry_demo/README.md` and the package tests as the source of truth for current behavior. Avoid adding new abstractions unless they reduce complexity in the existing dispatch/render pipeline.
