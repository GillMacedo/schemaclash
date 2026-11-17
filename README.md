# schemaclash: find OpenTelemetry Go "conflicting Schema URL" errors before they ship

[![CI](https://github.com/GillMacedo/schemaclash/actions/workflows/ci.yml/badge.svg)](https://github.com/GillMacedo/schemaclash/actions/workflows/ci.yml) [![MIT license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**A Claude Code skill and Go CLI for Go services that use OpenTelemetry. It reads your program's source and lists every function that merges resources with different schema URLs, which makes `resource.Merge` and `resource.New` fail with `conflicting Schema URL: https://opentelemetry.io/schemas/1.26.0 and https://opentelemetry.io/schemas/1.24.0`. Run it on a Dependabot or Renovate PR that bumps `go.opentelemetry.io/otel` and you find out before the service fails at startup.**

The OpenTelemetry Go getting-started code builds its resource like this:

```go
resource.Merge(resource.Default(),
	resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("api")))
```

`resource.Default()` carries the schema URL of the semconv version the SDK was built with. `semconv.SchemaURL` carries the one your import names. They match on the day you write the code. The next SDK upgrade moves `resource.Default()` to a newer semconv, your import stays where it was, and `Merge` returns an error. Most services treat that error as fatal, so the upgrade PR passes CI and the service doesn't start. Detectors such as `go.opentelemetry.io/contrib/detectors/gcp` have their own semconv import, so updating the SDK and a detector to releases built on different semconv versions does the same. In November 2026, GitHub's issue search found over 1,800 issues and pull requests that mention this error, from projects such as Loki, gcsfuse, OpenTofu and docker-agent.

schemaclash runs `go list -deps -export` (so the code must build), parses the source of every package that can reach `go.opentelemetry.io/otel/sdk/resource`, follows each schema URL into the functions that merge resources, and names the two versions and where each one comes from. It doesn't run your program.

## What you can ask Claude

- "Dependabot bumped go.opentelemetry.io/otel to v1.47.0. Will the service still start?"
- "Our service logs `conflicting Schema URL` at startup. Where does each schema come from?"
- "Which semconv version should this package import to match our OpenTelemetry SDK?"
- "Is it safe to upgrade go.opentelemetry.io/contrib/detectors/gcp without upgrading the SDK?"
- "Check this repo's tracing setup for OpenTelemetry resource conflicts."

## Install

As a Claude Code plugin:

    claude plugin marketplace add GillMacedo/schemaclash
    claude plugin install schemaclash@schemaclash-marketplace

Or run it with Go 1.22 or newer, in your module's directory. It has no dependencies outside the standard library:

    go run github.com/GillMacedo/schemaclash/plugin/scripts@v0.1.0 ./...

## Example

`examples/shop` is a made-up module with four programs, OpenTelemetry Go v1.32.0, whose `resource.Default()` uses semconv 1.26.0, and a `vendor/` directory, so it runs offline. `api` builds its resource with the getting-started pattern and an older semconv v1.24.0 import. `scanner` uses a shared detector still on semconv v1.21.0. `worker` and `billing` are fine. Running the programs prints this (log timestamps removed):

    api: telemetry: conflicting Schema URL: https://opentelemetry.io/schemas/1.26.0 and https://opentelemetry.io/schemas/1.24.0
    scanner: telemetry: error detecting resource: conflicting Schema URL: https://opentelemetry.io/schemas/1.21.0 and https://opentelemetry.io/schemas/1.26.0
    worker: started, resource schema https://opentelemetry.io/schemas/1.26.0
    billing: started, resource schema https://opentelemetry.io/schemas/1.20.0

schemaclash finds the same two without running anything. `--fail-on never` keeps the exit code at 0 for this demo; without it, the run exits 1:

    $ go run ./plugin/scripts --fail-on never --dir examples/shop
    schemaclash 0.1.0: examples/shop, 4 programs linking go.opentelemetry.io/otel/sdk v1.32.0; resource.Default uses schema 1.26.0

    ERROR schema-url-conflict  example.com/shop/cmd/scanner.main  cmd/scanner/main.go:15
      combines schema URLs 1.21.0 and 1.26.0 in one resource: resource.Merge, New and Detect return "conflicting Schema URL" here, and the resource they return has no schema URL
        1.26.0  resource.WithTelemetrySDK  cmd/scanner/main.go:19 (schema URL set at vendor/go.opentelemetry.io/otel/sdk/resource/builtin.go:54)
        1.26.0  resource.WithProcessPID  cmd/scanner/main.go:20 (schema URL set at vendor/go.opentelemetry.io/otel/sdk/resource/process.go:116)
        1.21.0  platform.Detector{}  cmd/scanner/main.go:18 (schema URL set at internal/platform/platform.go:22)
      programs: example.com/shop/cmd/scanner
      fix: in internal/platform/platform.go, import go.opentelemetry.io/otel/semconv/v1.26.0 instead of go.opentelemetry.io/otel/semconv/v1.21.0. Schema 1.26.0 is the one go.opentelemetry.io/otel/sdk v1.32.0 uses. Or build the extra attributes with resource.NewSchemaless, which has no schema URL

    ERROR schema-url-conflict  example.com/shop/internal/telemetry.NewResource  internal/telemetry/telemetry.go:12
      combines schema URLs 1.24.0 and 1.26.0 in one resource: resource.Merge, New and Detect return "conflicting Schema URL" here, and the resource they return has no schema URL
        1.26.0  resource.Default  internal/telemetry/telemetry.go:14 (schema URL set at vendor/go.opentelemetry.io/otel/sdk/resource/builtin.go:90)
        1.24.0  semconv.SchemaURL (go.opentelemetry.io/otel/semconv/v1.24.0)  internal/telemetry/telemetry.go:16
      programs: example.com/shop/cmd/api
      fix: in internal/telemetry/telemetry.go, import go.opentelemetry.io/otel/semconv/v1.26.0 instead of go.opentelemetry.io/otel/semconv/v1.24.0. Schema 1.26.0 is the one go.opentelemetry.io/otel/sdk v1.32.0 uses. Or build the extra attributes with resource.NewSchemaless, which has no schema URL

    2 errors: 2 functions of 6 that merge or detect resources mix schema URLs

The two programs that start are quiet. `billing` uses semconv 1.20.0, but only passes its resource to `sdktrace.WithResource`, which merges it with the environment's resource, and that one has no schema URL:

    $ go run ./plugin/scripts --dir examples/shop ./cmd/worker ./cmd/billing
    schemaclash 0.1.0: examples/shop, 2 programs linking go.opentelemetry.io/otel/sdk v1.32.0; resource.Default uses schema 1.26.0

    No conflicts: 3 functions that merge or detect resources, each with one schema URL.

The count includes functions in the SDK and other dependencies that merge resources, such as `sdktrace.WithResource`. Add `--format json` for machine-readable output. `examples/README.md` says how the example was made.

## In CI

Run it after the build step, so a dependency PR that would break startup fails:

    - run: go build ./...
    - run: go run github.com/GillMacedo/schemaclash/plugin/scripts@v0.1.0 ./...

Pass `--tags` with the build tags your release build uses. schemaclash checks the platform it runs on, like `go build`, so files for other platforms aren't read.

## What it reads

- **`go list -deps -export -json` in `--dir` (default `.`)** for the packages given (default `./...`). `-export` makes the go command compile every package, so code that doesn't build is refused. The go command uses `vendor/` or the module cache as it would for `go build`, and may download modules that aren't there.
- **The Go source of every package that can reach `go.opentelemetry.io/otel/sdk/resource`**, your own and your dependencies', including the SDK, plus the `SchemaURL` constant of each `go.opentelemetry.io/otel/semconv/vX` package. Test files aren't read.

## How it decides

A schema URL is `semconv.SchemaURL` from a `go.opentelemetry.io/otel/semconv/vX` import (named, renamed or dot-imported) or a string starting with `https://opentelemetry.io/schemas/`. It counts in a function that uses `sdk/resource`, or when it is passed to a function of a package that reaches `sdk/resource`. Schema URLs passed to other packages, such as `trace.WithSchemaURL` for a tracer, are left out: a tracer's schema URL is never merged with a resource's.

Schema URLs flow into a function through calls to functions and package variables that return a `*resource.Resource`, a `resource.Option`, a `resource.Detector` or a type with a `Detect` method; through a composite literal of a type with a `Detect` method (a detector, such as `platform.Detector{}`); and through method calls on values, matched by name within the same package. The SDK's own `resource.Default`, `WithTelemetrySDK`, `WithHost` and the others are read the same way from the SDK's source, so there is no table of versions to keep up to date. A function that calls `resource.Merge`, `New` or `Detect`, or a function that does, and receives two or more schema URLs, is a conflict. When a caller only repeats a conflict it calls, it is reported once, at the function that merges.

| Code | Severity | Meaning |
|---|---|---|
| `schema-url-conflict` | ERROR | This function merges resources whose schema URLs differ. `resource.Merge`, `New` and `Detect` return an error wrapping `resource.ErrSchemaURLConflict`, and the resource they return has no schema URL |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | No conflicts, or `--fail-on never` |
| 1 | At least one conflict |
| 2 | The input can't be trusted or nothing could be checked, or the flags are wrong. Stdout is empty, whatever `--fail-on` says |

## When it refuses

schemaclash exits 2 and prints only the reason when it can't trust what it read. For example, with a type error added to `examples/shop/cmd/api/main.go`, it prints this on stderr:

    schemaclash: can't trust the input, nothing was reported: go list -deps -export failed in /path/to/examples/shop, so the code doesn't build or the patterns are wrong: # example.com/shop/cmd/api; cmd/api/main.go:23:13: cannot use "x" (untyped string constant) as int value in variable declaration

It does the same for a `go.mod`, `vendor/modules.txt` or Go file (yours or vendored) that is empty, garbage, cut off, or replaced by a build log or an HTML error page; a missing import; missing modules with no way to download them; a directory that doesn't exist or has no `go.mod`; patterns that match nothing; packages that don't link `sdk/resource`; an SDK whose `resource.Default` it can't find a schema URL for; and a missing `go` command. A file that the go command doesn't read in that build, such as `go.sum` when building from `vendor/`, doesn't change the result.

## Limits

- **It follows names, not types.** Schema URLs that travel through struct fields, function values, interfaces other than detectors, channels or maps aren't followed. A method call on a value is matched by name within its package, so two types with a same-named method are read together.
- **One function is one resource.** A function that builds two resources with different schema URLs and merges neither, while it also calls `resource.Merge` or `resource.Default` elsewhere, is reported. Code that does that is rare, and the report names both lines.
- **Linked is not called.** A conflicting function in a dependency is reported for every program that links its package, whether or not the program calls it. Detectors that find nothing off their platform, such as the GCP detector off Google Cloud, only conflict where they run.
- **One platform per run.** It reads the files the go command builds for the current `GOOS`, `GOARCH` and `--tags`.
- **`OTEL_RESOURCE_ATTRIBUTES` and the environment** never carry a schema URL in the SDK, so they can't conflict and aren't read.
- **Collector and other languages** aren't covered. This is for Go programs that use `go.opentelemetry.io/otel/sdk`.

## FAQ

**What does "conflicting Schema URL" mean in OpenTelemetry Go?** Two resources being merged were built on different versions of the semantic conventions, so `resource.Merge` can't say which schema the result follows. It returns the merged attributes with no schema URL and an error wrapping `resource.ErrSchemaURLConflict`.

**How do I fix it?** Import the `go.opentelemetry.io/otel/semconv/vX` package whose version matches the one your SDK's `resource.Default()` uses (schemaclash prints it in the first line), or build your attributes with `resource.NewSchemaless`. For a detector from another module, move it and the SDK to releases built on the same semconv version.

**Why did it start after a Dependabot or Renovate PR?** The SDK moves `resource.Default()` to newer semconv versions in minor releases. Your code's import didn't move with it. A bump of a module such as gRPC can also raise the OpenTelemetry SDK through minimal version selection.

**Which semconv version does my OpenTelemetry SDK use?** Run schemaclash: the first line says `resource.Default uses schema X`. It reads that from the SDK's source in your module cache or `vendor/`.

**Does it run my code?** No. It runs `go list -deps -export`, which compiles packages into the build cache, and parses source files.

**Is it an MCP server?** No. It's a skill with one Go program, so `go run` or `go build` is all it needs.

## Tested on

- **The real SDK at run time.** `tests/live_test.go` runs every program in `examples/shop` and 16 small programs in `tests/testdata/cases` (10 that conflict and 6 that don't: matching versions, `NewSchemaless`, `WithResource` only, a tracer's schema URL, `WithFromEnv`, a detector on the SDK's version) with OpenTelemetry Go v1.32.0, and checks that schemaclash reports exactly the ones whose resource fails with `conflicting Schema URL`.
- **Real projects at the commit before their fix, by hand,** with Go 1.27.3 on linux/arm64 and the modules each one requires:
  - barnes-c/garmin_exporter at 6cfa90d~1 (issue 168): reported 1.41.0 against 1.43.0 in `buildResource`; quiet at the fix, 6cfa90d, and at e37a465.
  - GoogleCloudPlatform/gcsfuse v3.8.3 (issue 5162): reported `contrib/detectors/gcp` v1.44.0 (1.41.0) against SDK v1.45.0 (1.43.0) in `getResource`; quiet at v3.12.1.
  - grafana/loki v3.6.0 (issue 19975): reported `grafana/dskit` `tracing.NewResource`, 1.34.0 against 1.37.0, in 21 programs, the versions the issue quotes.
  - docker/docker-agent v1.44.0 (issue 2416): reported 1.37.0 against 1.40.0 in `initOTelSDK`, the versions the issue quotes; quiet at da10997.
  - opentofu/opentofu v1.11.10 and pomerium/ingress-controller: refused with exit 2, because `go build ./...` also fails here (a gRPC and x/net mismatch, and an envoy binary that `make` downloads).
- **Input it can't trust:** tests replace `go.mod`, `vendor/modules.txt`, a source file, a vendored SDK file and a vendored semconv file with empty, garbage, build-log and HTML content; cut those files at every line boundary and every seventh byte; and add a type error, a missing import, and the other refusals listed above. Each must exit 2 with nothing on stdout under every `--fail-on` value and both formats. A cut is allowed through only when `go build` accepts the cut tree too and the result is unchanged, as for trailing white space or `modules.txt` lines of packages only built on Windows.
- **The installed plugin:** a test copies `plugin/` out of the repository and builds it with the skill's own command.

Not tested: macOS and Windows; OpenTelemetry Go releases other than the ones these projects use (v1.32.0, v1.38.0, v1.43.0, v1.45.0, v1.46.0 and v1.47.0); `go.work` workspaces; cgo; a fresh Claude Code session using the skill. The `go run ...@v0.1.0` lines work only once the repository is published.

## Contributing

Contributions are welcome, from people and from people working with AI agents. See [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues are [listed here](https://github.com/GillMacedo/schemaclash/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

### How pull requests work

1. **Open a PR.** Fork the repo, work on a branch, and keep it to one change. Write a title that says what changes for users, because it becomes the changelog line: `feat:` goes under Added and `fix:` under Fixed.
2. **Automatic check.** Code changes need tests, and `make test` must pass. Changes over 30 lines need a linked open issue (`Closes #<number>`), and a PR can have at most 800 changed lines. If something is missing, a comment tells you what to fix.
3. **Review.** Claude reviews each PR automatically. Code PRs also get a security review and are tested in a sandbox. The review then merges the PR or asks for changes, and a small problem may become a follow-up issue instead. A PR with no new commit 14 days after a change request is closed, and you can reopen it. PRs that change CI, agent settings, npm or Python dependencies, vendored code or build files can't be merged this way, and a comment says so. The same goes for anything the security review doesn't clear.
4. **Release.** A merge that changes what users install becomes a new version within about two hours, with a GitHub Release and, if the project has one, its npm package. Docs and CI changes wait for the next release.

## AI disclosure

This project was largely generated with Claude Code and reviewed by its maintainer.

## About me

I'm Giulliano Macedo, and I build developer tools at GoTech Solutions. Follow along or get in touch:

- GitHub: [GillMacedo](https://github.com/GillMacedo)
- LinkedIn: [giulliano](https://www.linkedin.com/in/giulliano/)
- X: [@giumacedo](https://x.com/giumacedo)

If `schemaclash` saved you some prompts, a star helps others find it.

---

Built by [Giulliano Macedo](https://github.com/GillMacedo), GoTech Solutions · [LinkedIn](https://www.linkedin.com/in/giulliano/) · [X](https://x.com/giumacedo)
