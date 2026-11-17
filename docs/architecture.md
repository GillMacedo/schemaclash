# Architecture

schemaclash is one Go program in `plugin/scripts`, using only the standard library, so the installed plugin builds with `go build` and no module download.

## Flow

1. **Load (`load.go`).** `goList` runs `go list -deps -export -json=<fields> -- <patterns>` in `--dir`. `-export` compiles every package, so a syntax or type error, a broken `go.mod` or `vendor/modules.txt`, or a missing module makes the go command fail, and schemaclash exits 2 with its first error lines. `decodeList` reads the JSON stream and refuses anything that isn't whole objects, a package with an `Error` or `DepsErrors`, and an empty result.
2. **Parse (`analyze.go`).** `reaches` marks every package that imports `go.opentelemetry.io/otel/sdk/resource` directly or through others. Those packages, the SDK included, are parsed with `go/parser`. Each `go.opentelemetry.io/otel/semconv/vX` package's `SchemaURL` constant is read from its source.
3. **Scan.** For every package-level function, method and variable, `scan` records: whether it uses `sdk/resource`; whether it calls `Merge`, `New` or `Detect`; the schema URLs written in it (`semconv.SchemaURL`, or a string starting with `https://opentelemetry.io/schemas/`); and what it refers to: functions and variables by name, composite literals of types with a `Detect` method, and methods by name for calls on values. Schema URLs passed to packages that can't reach `sdk/resource` (such as `trace.WithSchemaURL`) are dropped, and schema URLs in a function that doesn't use `sdk/resource` count only when passed to a function of a package that does.
4. **Summarize.** `summarize` gives each declaration the set of schema URLs it can put into a resource: its own, plus those of what it refers to, following only functions whose results are a `*resource.Resource`, `resource.Option`, `resource.Detector` or a type with `Detect`. The SDK's `Default`, `WithTelemetrySDK`, `WithHost` and the rest are summarized from the SDK's source like any other code.
5. **Check (`check.go`).** A function outside `sdk/resource` that merges, directly or through a function it calls, and receives two or more schema URLs is a `schema-url-conflict`. A caller whose set equals a conflicting callee's is the same problem and is skipped. Each finding lists the main packages that link it. If no matched package links `sdk/resource`, or `resource.Default` doesn't summarize to exactly one schema URL, schemaclash refuses.
6. **Report (`schemaclash.go`).** Text or JSON. Exit 0, 1 by `--fail-on`, and 2 for every refusal.

## Why the runtime tests matter

The rules are approximations of the SDK's behavior. `tests/live_test.go` runs each example program with the vendored SDK and requires that schemaclash reports exactly the ones that print `conflicting Schema URL`.
