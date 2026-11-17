# 0002: Refuse code that doesn't build

## Context
Two earlier tools in this family passed their own tests and still reported a failed build, or an unreadable report, as all clear. A source reader that skips a file it can't parse, or reads a tree whose imports don't resolve, could report "no conflicts" for code that never ran.

## Decision
Use `-export`, so the go command compiles every package, and exit 2 with nothing on stdout, whatever `--fail-on` says, when `go list` fails, when its output isn't whole JSON or names a package error, when a file can't be parsed, when the patterns match nothing, when no package links `sdk/resource`, and when the SDK's default schema URL can't be found.

## Consequences
schemaclash can't be used on a branch that doesn't build; fix the build first. Tests replace and cut the example's files and accept a cut only when `go build` accepts it too and the result is unchanged.
