# Changelog

All notable changes are documented here, in [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. This project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-11-17

### Added
- `schemaclash` CLI and the `finding-otel-schema-url-conflicts` skill: reads a Go program's source through `go list -deps -export` and lists each function that merges OpenTelemetry resources with different schema URLs, the error `resource.Merge` and `resource.New` report as `conflicting Schema URL`.
- Follows schema URLs from `semconv.SchemaURL` and schema URL strings through functions, package variables, detectors, method calls and arguments, and reads the SDK's own `resource.Default` and built-in detectors from its source.
- `--dir`, `--tags`, `--format text|json` and `--fail-on error|never`.
- Refuses with exit 2 when the code doesn't build, when the patterns match nothing, when no package links `sdk/resource`, and when the SDK's default schema URL can't be found.
