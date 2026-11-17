Teach schemaclash a new way a schema URL reaches a resource: $ARGUMENTS

1. Write the smallest program that shows it in `tests/testdata/cases/<name>/main.go`, using only the packages vendored in `examples/shop`. It prints `conflict: <error>` when building the resource fails, and `ok <schema URL>` otherwise.
2. Add `<name>` to `expectConflict` in `tests/live_test.go` with what the program really prints, and run `go test ./tests -run TestCasesAgreeWithRuntime`.
3. Change `plugin/scripts/analyze.go` until the test passes. Add a quiet case next to it that must not be reported.
4. Update the README's "How it decides" and "Limits", then run `make test`.
