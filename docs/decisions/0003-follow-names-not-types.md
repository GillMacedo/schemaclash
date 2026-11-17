# 0003: Follow names, not types

## Context
Full type checking would need `go/types` and importers for every dependency, or golang.org/x/tools, which the dependency-free plugin can't have.

## Decision
Follow functions, package variables, detector types and same-package methods by name, and treat a function as one resource. Check every rule against the real SDK with a program in `tests/testdata/cases`.

## Consequences
Schema URLs carried through struct fields, function values or interfaces other than detectors are missed; the README lists this under Limits. A function that builds two separate resources with different versions and also merges something is reported.
