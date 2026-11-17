package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// failedBuild is real go build output, kept to check that a log pasted
// over a source file or go.mod is refused.
const failedBuild = `# example.com/shop/cmd/api
cmd/api/main.go:23:13: cannot use "x" (untyped string constant) as int value in variable declaration
`

// refused checks the contract for input schemaclash can't trust: exit 2,
// nothing on stdout, and a reason on stderr, under every --fail-on value
// and both formats.
func refused(t *testing.T, dir string, args ...string) {
	t.Helper()
	for _, extra := range [][]string{
		nil,
		{"--fail-on", "never"},
		{"--format", "json"},
		{"--format", "json", "--fail-on", "never"},
	} {
		r := runTool(t, dir, append(append([]string{}, extra...), args...)...)
		if r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "can't trust the input") {
			t.Fatalf("args %v: want exit 2, empty stdout and a reason; got exit %d\nstdout: %s\nstderr: %s", append(extra, args...), r.code, r.stdout, r.stderr)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestBadTrees changes one file of the example at a time.
func TestBadTrees(t *testing.T) {
	shopFiles := map[string]string{
		"go.mod":         "go.mod",
		"telemetry":      "internal/telemetry/telemetry.go",
		"main":           "cmd/api/main.go",
		"modules.txt":    "vendor/modules.txt",
		"sdk resource":   "vendor/go.opentelemetry.io/otel/sdk/resource/builtin.go",
		"semconv schema": "vendor/go.opentelemetry.io/otel/semconv/v1.24.0/schema.go",
	}
	contents := map[string]string{
		"empty":        "",
		"garbage":      "\x00\x01 this is not Go \xff\xfe\n{{{",
		"failed build": failedBuild,
		"html error":   "<html><body><h1>502 Bad Gateway</h1></body></html>\n",
	}
	for fname, rel := range shopFiles {
		for cname, content := range contents {
			fname, rel, cname, content := fname, rel, cname, content
			t.Run(fname+"/"+cname, func(t *testing.T) {
				t.Parallel()
				dir := shopCopy(t)
				writeFile(t, filepath.Join(dir, rel), content)
				refused(t, dir)
			})
		}
	}
}

// TestGoSumNotRead documents that with vendor/ the go command doesn't read
// go.sum, so a broken go.sum changes neither the build nor the result.
func TestGoSumNotRead(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "garbage": "\x00 not a go.sum {{{", "failed build": failedBuild} {
		dir := shopCopy(t)
		writeFile(t, filepath.Join(dir, "go.sum"), content)
		if !goBuildOK(t, dir) {
			t.Fatalf("%s: go build now fails, so this test is out of date", name)
		}
		rep, _ := runJSON(t, dir)
		if len(rep.Findings) != 2 {
			t.Fatalf("%s: want the same 2 findings, got %d", name, len(rep.Findings))
		}
	}
}

// goBuildOK reports whether go build accepts the module as it is.
func goBuildOK(t *testing.T, dir string) bool {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", os.DevNull, "./...")
	cmd.Dir = dir
	cmd.Env = goEnv()
	return cmd.Run() == nil
}

func TestBadSetups(t *testing.T) {
	t.Run("type error", func(t *testing.T) {
		dir := shopCopy(t)
		p := filepath.Join(dir, "cmd", "api", "main.go")
		writeFile(t, p, readFile(t, p)+"\nvar _ int = \"x\"\n")
		refused(t, dir)
	})
	t.Run("missing import", func(t *testing.T) {
		dir := shopCopy(t)
		p := filepath.Join(dir, "internal", "telemetry", "telemetry.go")
		writeFile(t, p, strings.Replace(readFile(t, p), `semconv/v1.24.0"`, `semconv/v1.99.0"`, 1))
		refused(t, dir)
	})
	t.Run("not a directory", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file")
		writeFile(t, f, "module x\n")
		refused(t, t.TempDir(), "--dir", f)
	})
	t.Run("missing directory", func(t *testing.T) {
		refused(t, t.TempDir(), "--dir", filepath.Join(t.TempDir(), "nope"))
	})
	t.Run("empty directory", func(t *testing.T) {
		refused(t, t.TempDir())
	})
	t.Run("no go.mod", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
		refused(t, dir)
	})
	t.Run("no OpenTelemetry", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/plain\n\ngo 1.22\n")
		writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
		refused(t, dir)
	})
	t.Run("pattern matches nothing", func(t *testing.T) {
		dir := shopCopy(t)
		writeFile(t, filepath.Join(dir, "empty", "README"), "no Go here\n")
		refused(t, dir, "./empty/...")
	})
	t.Run("pattern for missing directory", func(t *testing.T) {
		refused(t, shopCopy(t), "./nothing/...")
	})
	t.Run("only packages without OpenTelemetry", func(t *testing.T) {
		dir := shopCopy(t)
		writeFile(t, filepath.Join(dir, "cmd", "plain", "main.go"), "package main\n\nfunc main() {}\n")
		refused(t, dir, "./cmd/plain")
	})
	t.Run("SDK laid out differently", func(t *testing.T) {
		// If resource.Default stops carrying a schema URL that can be
		// found, schemaclash can't know what to compare with.
		dir := shopCopy(t)
		p := filepath.Join(dir, "vendor", "go.opentelemetry.io", "otel", "sdk", "resource", "builtin.go")
		src := readFile(t, p)
		src = strings.ReplaceAll(src, "semconv.SchemaURL,", `"",`)
		src = strings.ReplaceAll(src, "StringDetector(semconv.SchemaURL,", `StringDetector("",`)
		writeFile(t, p, src)
		refused(t, dir)
	})
	t.Run("go not on PATH", func(t *testing.T) {
		dir := shopCopy(t)
		r := runToolEnv(t, dir, []string{"PATH=" + t.TempDir()})
		if r.code != 2 || r.stdout != "" {
			t.Fatalf("exit %d, stdout %q", r.code, r.stdout)
		}
	})
	t.Run("missing module cache", func(t *testing.T) {
		dir := shopCopy(t)
		if err := os.RemoveAll(filepath.Join(dir, "vendor")); err != nil {
			t.Fatal(err)
		}
		r := runToolEnv(t, dir, []string{"GOMODCACHE=" + t.TempDir(), "GOFLAGS=-mod=mod"})
		if r.code != 2 || r.stdout != "" {
			t.Fatalf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
		}
	})
	t.Run("bad flags", func(t *testing.T) {
		dir := shopCopy(t)
		for _, args := range [][]string{{"--format", "xml"}, {"--fail-on", "warn"}, {"--nope"}} {
			r := runTool(t, dir, args...)
			if r.code != 2 || r.stdout != "" {
				t.Fatalf("%v: exit %d, stdout %q", args, r.code, r.stdout)
			}
		}
	})
}

func runToolEnv(t *testing.T, dir string, env []string) result {
	t.Helper()
	return runToolWith(t, dir, env)
}

// TestCutFiles cuts files of the example short. A cut is refused, unless
// go build accepts the cut tree too, and then the result must be the same
// as for the whole file. That happens for trailing white space, and for
// vendor/modules.txt lines of packages this platform doesn't build.
func TestCutFiles(t *testing.T) {
	full := runTool(t, filepath.Join(repoRoot, "examples", "shop"), "--format", "json")
	if full.code != 1 {
		t.Fatalf("the uncut example should exit 1, got %d: %s", full.code, full.stderr)
	}
	for _, rel := range []string{"go.mod", "internal/telemetry/telemetry.go", "internal/platform/platform.go", "vendor/modules.txt"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			dir := shopCopy(t)
			path := filepath.Join(dir, rel)
			orig := readFile(t, path)
			for _, n := range cutPoints(orig) {
				writeFile(t, path, orig[:n])
				r := runTool(t, dir, "--format", "json")
				if r.code == 2 && r.stdout == "" {
					continue
				}
				if !goBuildOK(t, dir) {
					t.Fatalf("cut at %d of %d was accepted, and go build refuses it: exit %d\n%s", n, len(orig), r.code, r.stdout)
				}
				if r.code != full.code || r.stdout != full.stdout {
					t.Fatalf("cut at %d of %d builds, and changed the result: exit %d\n%s%s", n, len(orig), r.code, r.stdout, r.stderr)
				}
			}
			writeFile(t, path, orig)
		})
	}
}

// cutPoints returns every line start and end, the bytes around them, and
// every seventh byte, so long files stay fast to test.
func cutPoints(s string) []int {
	seen := map[int]bool{}
	var out []int
	add := func(n int) {
		if n >= 0 && n < len(s) && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	add(0)
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			add(i - 1)
			add(i)
			add(i + 1)
		}
		if i%7 == 0 {
			add(i)
		}
	}
	add(len(s) - 1)
	return out
}
