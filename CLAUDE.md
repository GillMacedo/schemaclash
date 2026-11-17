# schemaclash

Claude Code skill and Go CLI that finds OpenTelemetry Go code merging resources with different schema URLs, which fails with `conflicting Schema URL` (`resource.ErrSchemaURLConflict`).

## Architecture
`plugin/scripts/schemaclash.go` holds the CLI and the text report. `plugin/scripts/load.go` runs `go list -deps -export -json` and decodes it strictly (`goList`, `decodeList`). `plugin/scripts/analyze.go` parses every package that can reach `go.opentelemetry.io/otel/sdk/resource` and summarizes the schema URLs each function, variable and method can put into a resource (`scan`, `summarize`, `inputs`). `plugin/scripts/check.go` finds the functions that merge two or more of them (`check`, `fixText`). `plugin/skills/finding-otel-schema-url-conflicts/SKILL.md` tells Claude how to use it. See `docs/architecture.md`.

## Commands
- `make test`: gofmt, vet and tests, including `tests/`, which run the example programs and the cases in `tests/testdata/cases` with the real SDK and compare
- `make lint`: plugin manifest validation (needs Claude Code)
- `make demo`: run on `examples/shop`

## Conventions
- Standard library only. Never run the user's program or change their files; `go list -deps -export` is the only command run.
- Any input it can't trust exits 2 with nothing on stdout, whatever `--fail-on` says. Add a case to `TestBadTrees` or `TestBadSetups` in `tests/inputs_test.go` for each new kind of input.
- A new rule about how a schema URL reaches a resource needs a program in `tests/testdata/cases` that the test runs, with an entry in `expectConflict`, and a healthy program that must stay quiet.
- Don't hard-code SDK versions or semconv versions; read them from the SDK's source.
- Tests must not assume the machine they run on: use `runtime` or `go env` for anything platform-specific.

## Do and don't
- Do run `make test` on Go 1.22 and on the current Go release before calling a change done. `GOTOOLCHAIN=go1.22.12 make test` works.
- Don't add `go.mod` requirements to the root module: the installed plugin has no module.
- Don't print "No conflicts" for a run that checked nothing.
- Don't change `examples/shop` without updating the README output; `tests/readme_test.go` compares them.

## Definition of done
`make test` and `make lint` pass, README output matches a real run, and `ROADMAP.md` and docs are updated when relevant.
