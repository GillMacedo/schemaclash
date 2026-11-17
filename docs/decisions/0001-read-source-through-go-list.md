# 0001: Read the source through go list, with no version table

## Context
The schema URL `resource.Default()` uses changes with the SDK version, and detector modules each have their own. A table of versions would go stale with every SDK release.

## Decision
Run `go list -deps -export` and read the SDK's, the detectors' and the program's source from the paths it reports. The SDK's default schema URL is whatever its own source summarizes to.

## Consequences
schemaclash needs the module's dependencies (module cache or `vendor/`) and code that builds. It follows any SDK release without changes, unless the SDK's layout changes so much that `resource.Default` no longer summarizes to one schema URL, and then it refuses instead of guessing.
