# Examples

`shop` is a made-up Go module with four programs that use OpenTelemetry Go v1.32.0. That SDK's `resource.Default()` uses semconv 1.26.0.

| Program | Resource setup | When run |
|---|---|---|
| `cmd/api` | `internal/telemetry.NewResource`: `resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, ...))` with semconv v1.24.0 | exits 1 with `conflicting Schema URL` |
| `cmd/scanner` | `resource.New` with `internal/platform.Detector` (semconv v1.21.0), `WithTelemetrySDK` and `WithProcessPID` | exits 1 with `conflicting Schema URL` |
| `cmd/worker` | `resource.New` with `WithTelemetrySDK`, `WithHost` and semconv v1.26.0 | starts |
| `cmd/billing` | `resource.NewWithAttributes` with semconv v1.20.0, passed only to `sdktrace.WithResource` | starts |

It was made with:

    go mod init example.com/shop
    go get go.opentelemetry.io/otel@v1.32.0 go.opentelemetry.io/otel/sdk@v1.32.0
    go mod tidy
    go mod vendor

`vendor/` holds the dependencies, under their own licenses (Apache-2.0 for OpenTelemetry Go, BSD-3-Clause for golang.org/x/sys and google/uuid, Apache-2.0 for go-logr), so the example and the tests run offline. Run a program with `go run ./cmd/api` in this directory, and schemaclash with `go run ./plugin/scripts --dir examples/shop` from the repository root.
