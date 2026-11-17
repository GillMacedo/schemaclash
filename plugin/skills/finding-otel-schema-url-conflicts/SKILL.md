---
name: finding-otel-schema-url-conflicts
description: Finds OpenTelemetry Go code that fails with "conflicting Schema URL" (resource.ErrSchemaURLConflict) by reading the program's source through go list. Catches resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, ...)) where the semconv import no longer matches the SDK's version, resource.New with WithTelemetrySDK, WithHost or WithProcess next to WithSchemaURL from another semconv version, and detectors (go.opentelemetry.io/contrib/detectors, or your own) built on another semconv version. Use when a Go service logs or exits with "conflicting Schema URL", when reviewing a Dependabot or Renovate PR that bumps go.opentelemetry.io/otel, otel/sdk, contrib or grpc, when changing tracing or metrics setup, and before upgrading OpenTelemetry Go.
---

# Finding OpenTelemetry schema URL conflicts in Go

schemaclash runs `go list -deps -export` on the packages you give it (default `./...`), reads the source of every package that can reach `go.opentelemetry.io/otel/sdk/resource`, and lists each function that combines resources whose schema URLs differ. `resource.Merge`, `resource.New` and `resource.Detect` return `conflicting Schema URL` there, and the resource they return has no schema URL. It doesn't run the program. It needs Go 1.22 or newer, and the module's dependencies in the module cache or `vendor/` (the go command downloads them if it can).

## Run it

Build it once, then run it in the module's directory:

    go build -o "${TMPDIR:-/tmp}/schemaclash" $(ls "${CLAUDE_PLUGIN_ROOT}"/scripts/*.go | grep -v _test.go)
    "${TMPDIR:-/tmp}/schemaclash" --dir . ./...

Pass the same build tags CI uses with `--tags`. Use `--format json` for machine-readable output. To check a dependency upgrade before merging it, run it on the branch that has the new `go.mod`.

## Read the exit code first

- **0:** no conflicts. The last line says how many functions that merge or detect resources were checked.
- **1:** at least one conflict.
- **2:** nothing was reported. The message says why: the code doesn't build (`go list -deps -export` failed: a syntax or type error, a broken `go.mod` or `vendor/modules.txt`, a missing module), the patterns matched nothing, or no package links `sdk/resource`. Fix that and run again. Never describe exit 2 as a clean result.

## Read each finding

`ERROR schema-url-conflict` names the function that merges, where it is, and each schema URL that reaches it, with the line where it enters the function and where the URL is set. The usual causes:

- **The SDK was upgraded, the semconv import wasn't.** `resource.Default()`, `WithTelemetrySDK()`, `WithHost()`, `WithOS()`, `WithProcess*()` and `WithContainer*()` use the semconv version of the SDK. Code that still imports an older `go.opentelemetry.io/otel/semconv/vX` and passes `semconv.SchemaURL` to `NewWithAttributes` or `WithSchemaURL` conflicts. Fix: import the version the header line names.
- **A detector on another version.** `go.opentelemetry.io/contrib/detectors/...` and other detector modules have their own semconv import. The finding says `(in a dependency)` or names the module. Fix: move that module to the release built on the same SDK version, or the SDK to the one the detector expects.
- **Two versions in your own code.** Use one.

`resource.NewSchemaless` builds attributes without a schema URL, which never conflicts.

## Report

1. Each conflict: the function, the two versions, and where each comes from.
2. The fix the finding gives, in terms of the user's files or `go.mod` lines.
3. Which programs link it (the `programs:` line), so the user knows what fails at startup.

## Don't

Don't call the code safe because schemaclash found nothing: it follows functions, package variables, detectors and methods by name, not every value through interfaces, function values or struct fields (see the README's Limits). Don't change `go.mod` versions without asking; name the change and let the user decide. If a program exits with `conflicting Schema URL` and schemaclash reports nothing, say so and suggest opening an issue with the resource setup code.
