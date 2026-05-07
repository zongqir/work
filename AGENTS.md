# Repository Guidelines

## Project Structure & Module Organization
This repository is a workspace repo named `work`. The current Go module lives under `notification/`. Its implementation is under `notification/code/`, with packages organized by responsibility: `contract/`, `config/`, `dispatch/`, `delivery/`, `consumer/`, `render/`, `scheduler/`, `publisher/`, `preview/`, `bootstrap/`, and `handlers/<message_type>/`. Module-specific requirement documents live in `notification/*.md`.

## Build, Test, and Development Commands
- `cd notification; go test ./...` runs the full test suite for the notification module.
- `cd notification; go build ./...` verifies the notification module compiles.
- `cd notification/code; go test ./preview` runs the preview-focused package tests.

## Coding Style & Naming Conventions
Use standard Go formatting (`gofmt`) and idiomatic Go naming: short package names, exported identifiers in `CamelCase`, unexported identifiers in `camelCase`, and tests in `*_test.go`. Keep package boundaries narrow and prefer one `handler.go` per `handlers/<message_type>/` directory. JSON/template fixture names should stay descriptive and aligned with the message type, such as `xdr_risk_digest_default.body.tmpl`.

## Testing Guidelines
Tests use Go’s built-in `testing` package. Name tests `TestXxx` and keep package-local fakes/stubs close to the package under test. Prefer table-driven tests for config, rendering, and dispatch logic. When changing behavior, run `go test ./...` before submitting.

## Commit & Pull Request Guidelines
Recent commits in this repository use short, minimal subjects, so keep commit messages concise and imperative when possible. PRs should include a brief summary, the affected package(s), and validation notes (`go test ./...`, `go build ./...`, or package-specific tests). Add sample output or screenshots only when the change affects rendered templates or generated artifacts.

## Agent Notes
Treat `notification/code/README.md` and the package tests as the source of truth for current behavior. Avoid adding new abstractions unless they reduce complexity in the existing dispatch/render pipeline.

