package tests

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestExampleShopAgreesWithRuntime runs every program in examples/shop and
// checks that schemaclash reports exactly the ones that fail to build
// their resource.
func TestExampleShopAgreesWithRuntime(t *testing.T) {
	dir := filepath.Join(repoRoot, "examples", "shop")
	rep, _ := runJSON(t, dir)
	flagged := map[string]bool{}
	for _, f := range rep.Findings {
		for _, p := range f.Programs {
			flagged[p] = true
		}
	}
	want := map[string]bool{"api": true, "scanner": true, "worker": false, "billing": false}
	for name, conflict := range want {
		pkg := "example.com/shop/cmd/" + name
		out, err := runProgram(t, dir, "./cmd/"+name)
		failed := err != nil && strings.Contains(out, "conflicting Schema URL")
		if failed != conflict {
			t.Errorf("%s: expected the program to fail with a conflict: %v, got err=%v, output %q", name, conflict, err, out)
		}
		if !failed && err != nil {
			t.Errorf("%s failed for another reason: %v\n%s", name, err, out)
		}
		if flagged[pkg] != conflict {
			t.Errorf("%s: schemaclash flagged=%v, the real program conflicts=%v", name, flagged[pkg], conflict)
		}
	}
	if rep.SDK.DefaultSchemaURL != "https://opentelemetry.io/schemas/1.26.0" {
		t.Errorf("SDK schema URL = %q", rep.SDK.DefaultSchemaURL)
	}
	if len(rep.Programs) != 4 {
		t.Errorf("programs = %v, want 4", rep.Programs)
	}
}

// expectConflict is what each program in testdata/cases does when run:
// true if building its resource fails with ErrSchemaURLConflict.
var expectConflict = map[string]bool{
	"aliased":       true,
	"detectorptr":   true,
	"dotimport":     true,
	"literal":       true,
	"mergedefault":  true,
	"method":        true,
	"newoptions":    true,
	"packagevar":    true,
	"param":         true,
	"twoversions":   true,
	"detectormatch": false,
	"fromenv":       false,
	"matching":      false,
	"schemaless":    false,
	"tracerscope":   false,
	"withresource":  false,
}

// TestCasesAgreeWithRuntime copies each case into the shop module, runs it
// with the real SDK, and checks that schemaclash reports a conflict for
// exactly the programs that hit one.
func TestCasesAgreeWithRuntime(t *testing.T) {
	dir := shopCopy(t)
	casesDir := filepath.Join(repoRoot, "tests", "testdata", "cases")
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
			copyTree(t, filepath.Join(casesDir, e.Name()), filepath.Join(dir, "cmd", e.Name()))
		}
	}
	sort.Strings(names)
	if len(names) != len(expectConflict) {
		t.Fatalf("testdata/cases has %d cases, expectConflict lists %d", len(names), len(expectConflict))
	}
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			want, ok := expectConflict[name]
			if !ok {
				t.Fatalf("case %s is missing from expectConflict", name)
			}
			out, err := runProgram(t, dir, "./cmd/"+name)
			if err != nil {
				t.Fatalf("running the case: %v\n%s", err, out)
			}
			ranConflict := strings.HasPrefix(out, "conflict:") && strings.Contains(out, "conflicting Schema URL")
			if ranConflict != want {
				t.Fatalf("the real program printed %q; the test expects conflict=%v", out, want)
			}
			rep, _ := runJSON(t, dir, "./cmd/"+name+"/...")
			if got := len(rep.Findings) > 0; got != want {
				t.Fatalf("schemaclash reported %d findings, the real program printed %q", len(rep.Findings), out)
			}
			r := runTool(t, dir, "./cmd/"+name+"/...")
			wantCode := 0
			if want {
				wantCode = 1
			}
			if r.code != wantCode {
				t.Fatalf("exit %d without --fail-on, want %d\n%s%s", r.code, wantCode, r.stdout, r.stderr)
			}
			if !want && !strings.Contains(r.stdout, "No conflicts:") {
				t.Fatalf("a clean run should say so:\n%s", r.stdout)
			}
			// Every schema URL the program printed in its error must be
			// one the finding names.
			if want {
				for _, u := range rep.Findings[0].SchemaURLs {
					if !strings.Contains(out, u) {
						t.Errorf("finding names %s, the program's error doesn't: %q", u, out)
					}
				}
			}
		})
	}
}
