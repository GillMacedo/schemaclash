// Command schemaclash finds the places in a Go program that merge
// OpenTelemetry resources with different schema URLs, which makes
// resource.Merge and resource.New return "conflicting Schema URL", usually
// at startup and right after an SDK upgrade.
//
// It runs go list -deps -export on the packages given (default ./...) and
// reads the source of every package that can reach
// go.opentelemetry.io/otel/sdk/resource. It doesn't run the program.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("schemaclash", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "module directory to run go list in")
	tags := fs.String("tags", "", "build tags, passed to go list -tags")
	format := fs.String("format", "text", "output format: text or json")
	failOn := fs.String("fail-on", "error", "exit 1 on findings (error), or never")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: schemaclash [--dir DIR] [--tags TAGS] [--format text|json] [--fail-on error|never] [packages]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "schemaclash", version)
		return 0
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "schemaclash: --format must be text or json, not %q\n", *format)
		return 2
	}
	if *failOn != "error" && *failOn != "never" {
		fmt.Fprintf(stderr, "schemaclash: --fail-on must be error or never, not %q\n", *failOn)
		return 2
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	rep, err := analyze(*dir, *tags, patterns)
	if err != nil {
		fmt.Fprintf(stderr, "schemaclash: can't trust the input, nothing was reported: %v\n", err)
		return 2
	}
	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if rep.Findings == nil {
			rep.Findings = []finding{}
		}
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintf(stderr, "schemaclash: %v\n", err)
			return 2
		}
	} else {
		writeText(stdout, rep)
	}
	if len(rep.Findings) > 0 && *failOn == "error" {
		return 1
	}
	return 0
}

func analyze(dir, tags string, patterns []string) (*report, error) {
	if !isDir(dir) {
		return nil, refuse("--dir %s is not a directory", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, refuse("--dir %s: %v", dir, err)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	list, err := goList(abs, tags, patterns)
	if err != nil {
		return nil, err
	}
	rep, err := check(abs, list)
	if err != nil {
		return nil, err
	}
	rep.Dir = filepath.ToSlash(dir)
	return rep, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func writeText(w io.Writer, rep *report) {
	fmt.Fprintf(w, "%s: %s, %s linking %s; resource.Default uses schema %s\n",
		rep.Tool, rep.Dir, plural(len(rep.Programs), "program", "programs"), rep.SDK.Module, ver(rep.SDK.SchemaURL))
	for _, f := range rep.Findings {
		fmt.Fprintln(w)
		where := ""
		if f.Dependency {
			where = " (in a dependency)"
		}
		fmt.Fprintf(w, "ERROR %s  %s  %s%s\n", f.Code, f.Function, f.Place, where)
		vs := make([]string, len(f.SchemaURLs))
		for i, u := range f.SchemaURLs {
			vs[i] = ver(u)
		}
		fmt.Fprintf(w, "  combines schema URLs %s in one resource: resource.Merge, New and Detect return \"conflicting Schema URL\" here, and the resource they return has no schema URL\n", strings.Join(vs, " and "))
		for _, in := range f.Inputs {
			if in.Via == "" {
				fmt.Fprintf(w, "    %s  %s  %s\n", ver(in.URL), in.Origin.What, in.At)
				continue
			}
			fmt.Fprintf(w, "    %s  %s  %s (schema URL set at %s)\n", ver(in.URL), in.Via, in.At, in.Origin.Place)
		}
		if len(f.Programs) > 0 {
			fmt.Fprintf(w, "  programs: %s\n", strings.Join(f.Programs, " "))
		} else {
			fmt.Fprintf(w, "  programs: none of the packages checked link it, but a call to it fails the same way\n")
		}
		fmt.Fprintf(w, "  fix: %s\n", f.Fix)
	}
	fmt.Fprintln(w)
	if len(rep.Findings) == 0 {
		fmt.Fprintf(w, "No conflicts: %s that merge or detect resources, each with one schema URL.\n", plural(rep.MergePoints, "function", "functions"))
		return
	}
	fmt.Fprintf(w, "%s: %s of %d that merge or detect resources mix schema URLs\n",
		plural(len(rep.Findings), "error", "errors"), plural(len(rep.Findings), "function", "functions"), rep.MergePoints)
}
