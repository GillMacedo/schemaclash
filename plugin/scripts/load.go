package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// listModule is the module part of go list -json output.
type listModule struct {
	Path    string
	Version string
	Main    bool
	Dir     string
	Replace *listModule
}

// listPkg is the part of go list -json output schemaclash uses.
type listPkg struct {
	ImportPath string
	Name       string
	Dir        string
	Standard   bool
	DepOnly    bool
	GoFiles    []string
	CgoFiles   []string
	Imports    []string
	Module     *listModule
	Error      *struct{ Err string }
	DepsErrors []*struct{ Err string }
	Incomplete bool
}

const listFields = "ImportPath,Name,Dir,Standard,DepOnly,GoFiles,CgoFiles,Imports,Module,Error,DepsErrors,Incomplete"

// untrusted marks an error that means the input can't be trusted, so
// nothing is reported and the exit code is 2.
type untrusted struct{ msg string }

func (e untrusted) Error() string { return e.msg }

func refuse(format string, args ...any) error {
	return untrusted{fmt.Sprintf(format, args...)}
}

// goList runs go list -deps -export in dir. -export makes the go command
// compile every package, so code that doesn't build is refused here rather
// than analyzed.
func goList(dir, tags string, patterns []string) ([]*listPkg, error) {
	args := []string{"list", "-deps", "-export", "-json=" + listFields}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "--")
	args = append(args, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return nil, refuse("can't run the go command: %v", err)
		}
		return nil, refuse("go list -deps -export failed in %s, so the code doesn't build or the patterns are wrong: %s", dir, firstLines(stderr.String(), 6))
	}
	return decodeList(&stdout)
}

// decodeList reads the stream of JSON objects go list -json prints.
func decodeList(r io.Reader) ([]*listPkg, error) {
	dec := json.NewDecoder(r)
	var pkgs []*listPkg
	for {
		var p listPkg
		err := dec.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, refuse("go list printed output that isn't complete JSON: %v", err)
		}
		if p.ImportPath == "" {
			return nil, refuse("go list printed a package with no import path")
		}
		if p.Error != nil {
			return nil, refuse("package %s has an error: %s", p.ImportPath, oneLine(p.Error.Err))
		}
		for _, e := range p.DepsErrors {
			if e != nil {
				return nil, refuse("a dependency of %s has an error: %s", p.ImportPath, oneLine(e.Err))
			}
		}
		if p.Incomplete {
			return nil, refuse("go list says package %s is incomplete", p.ImportPath)
		}
		pkgs = append(pkgs, &p)
	}
	if len(pkgs) == 0 {
		return nil, refuse("go list found no packages")
	}
	return pkgs, nil
}

// firstLines keeps the first n lines of go command errors, leaving out
// its "go: downloading" progress lines.
func firstLines(s string, n int) string {
	var keep []string
	for _, l := range strings.Split(s, "\n") {
		if !strings.HasPrefix(l, "go: downloading ") {
			keep = append(keep, l)
		}
	}
	s = strings.TrimSpace(strings.Join(keep, "\n"))
	if s == "" {
		return "(no message)"
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = append(lines[:n], fmt.Sprintf("(%d more lines)", len(lines)-n))
	}
	return strings.Join(lines, "; ")
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
