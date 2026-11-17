// Package tests runs schemaclash against real Go modules: the example in
// examples/shop, small programs in testdata/cases that the tests also run,
// and broken copies of both.
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var (
	binary   string
	repoRoot string
)

func TestMain(m *testing.M) {
	os.Exit(testMain(m))
}

func testMain(m *testing.M) int {
	root, err := filepath.Abs("..")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	repoRoot = root
	tmp, err := os.MkdirTemp("", "schemaclash-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(tmp)
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	binary = filepath.Join(tmp, "schemaclash"+exe)
	build := exec.Command("go", "build", "-o", binary, "./plugin/scripts")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building schemaclash: %v\n%s", err, out)
		return 1
	}
	return m.Run()
}

// goEnv keeps the go command offline and on the toolchain running the
// tests, so a broken go.mod can't make it download anything.
func goEnv() []string {
	return append(os.Environ(), "GOPROXY=off", "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off")
}

type result struct {
	stdout string
	stderr string
	code   int
}

func runTool(t *testing.T, dir string, args ...string) result {
	t.Helper()
	return runToolWith(t, dir, nil, args...)
}

// runToolWith runs schemaclash with extra environment variables, which
// override goEnv's.
func runToolWith(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(goEnv(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running schemaclash: %v", err)
		}
		code = ee.ExitCode()
	}
	return result{stdout.String(), stderr.String(), code}
}

type jsonInput struct {
	SchemaURL string `json:"schemaURL"`
	Via       string `json:"via"`
}

type jsonFinding struct {
	Code       string      `json:"code"`
	Function   string      `json:"function"`
	SchemaURLs []string    `json:"schemaURLs"`
	Programs   []string    `json:"programs"`
	Inputs     []jsonInput `json:"inputs"`
	Dependency bool        `json:"inDependency"`
}

type jsonReport struct {
	Programs    []string `json:"programs"`
	MergePoints int      `json:"mergePoints"`
	SDK         struct {
		Module           string `json:"module"`
		DefaultSchemaURL string `json:"defaultSchemaURL"`
	} `json:"sdk"`
	Findings []jsonFinding `json:"findings"`
}

func runJSON(t *testing.T, dir string, args ...string) (jsonReport, int) {
	t.Helper()
	r := runTool(t, dir, append([]string{"--format", "json", "--fail-on", "never"}, args...)...)
	if r.code != 0 {
		t.Fatalf("schemaclash %v in %s: exit %d\n%s", args, dir, r.code, r.stderr)
	}
	var rep jsonReport
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, r.stdout)
	}
	return rep, r.code
}

// copyTree copies a directory, so tests can change files in the copy.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying %s: %v", src, err)
	}
}

// shopCopy returns a fresh copy of examples/shop.
func shopCopy(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "shop")
	copyTree(t, filepath.Join(repoRoot, "examples", "shop"), dir)
	return dir
}

// runProgram runs one of the shop's programs for real and returns what it
// printed.
func runProgram(t *testing.T, dir, pkg string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "run", pkg)
	cmd.Dir = dir
	cmd.Env = goEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}
