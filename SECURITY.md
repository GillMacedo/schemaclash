# Security

Report vulnerabilities through [GitHub's private vulnerability reporting](https://github.com/GillMacedo/schemaclash/security/advisories/new), not in a public issue.

`schemaclash` runs `go list -deps -export` in the directory you give it and reads Go source files. `go list -export` compiles packages into the Go build cache and, like `go build`, may download modules that aren't in the module cache or `vendor/`; with cgo it runs the C compiler. schemaclash doesn't run your program, write to your files, or connect anywhere itself. Function names, file paths and module versions are printed in findings, so don't share its output if those hold anything private.

A missed or wrong finding is a normal bug. If `schemaclash` changes a file, runs code from the module, or reports a clean result for code it couldn't read, report it privately. `schemaclash` is a diagnostic aid and doesn't enforce anything.
