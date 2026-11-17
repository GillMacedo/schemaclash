# Roadmap

## 0.1.0 (this release)
- List the functions in a Go program that merge OpenTelemetry resources with different schema URLs, with where each URL comes from and the fix.

## Next
- A `--sdk` what-if: say whether upgrading `go.opentelemetry.io/otel/sdk` to a given version would cause a conflict, before the upgrade is made.
- Follow schema URLs through struct fields, such as a `Config{SchemaURL: semconv.SchemaURL}` passed to a helper.
- Say when the code drops the error from `resource.Merge` or `resource.New`, so the conflict only loses the schema URL instead of failing.
- SARIF output for GitHub code scanning.
- Check every platform in one run, the way `GOOS=... go list` would.

## Not planned
- Running the program or connecting to a collector.
- Changing `go.mod` or source files. schemaclash names the change and leaves it to you.
