package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const skillRel = "plugin/skills/finding-otel-schema-url-conflicts/SKILL.md"

// TestInstalledPlugin copies plugin/ to a directory outside this module, the
// way Claude Code installs it, and builds it with the command the skill uses.
// The plugin has no go.mod, so it must build from its .go files alone.
func TestInstalledPlugin(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "schemaclash")
	copyTree(t, filepath.Join(repoRoot, "plugin"), root)
	const buildLine = `go build -o "${TMPDIR:-/tmp}/schemaclash" $(ls "${CLAUDE_PLUGIN_ROOT}"/scripts/*.go | grep -v _test.go)`
	if !strings.Contains(readFile(t, filepath.Join(repoRoot, skillRel)), buildLine) {
		t.Fatalf("SKILL.md doesn't contain the build line this test runs:\n%s", buildLine)
	}
	if runtime.GOOS == "windows" {
		t.Skip("the skill's build line is for a POSIX shell")
	}
	out := filepath.Join(dir, "tmp")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", buildLine)
	cmd.Dir = dir
	cmd.Env = append(goEnv(), "CLAUDE_PLUGIN_ROOT="+root, "TMPDIR="+out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the installed plugin failed: %v\n%s", err, b)
	}
	run := exec.Command(filepath.Join(out, "schemaclash"), "--dir", filepath.Join(repoRoot, "examples", "shop"))
	run.Env = goEnv()
	b, err := run.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 || !strings.Contains(string(b), "2 errors:") {
		t.Fatalf("the installed plugin's binary gave %v:\n%s", err, b)
	}
}

func TestManifests(t *testing.T) {
	var plugin struct{ Name, Version string }
	var market struct {
		Name    string
		Plugins []struct{ Name, Source string }
	}
	for p, v := range map[string]any{"plugin/.claude-plugin/plugin.json": &plugin, ".claude-plugin/marketplace.json": &market} {
		if err := json.Unmarshal([]byte(readFile(t, filepath.Join(repoRoot, p))), v); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if plugin.Name != "schemaclash" || plugin.Version != "0.1.0" {
		t.Errorf("plugin.json: %+v", plugin)
	}
	if market.Name != "schemaclash-marketplace" || len(market.Plugins) != 1 || market.Plugins[0].Name != "schemaclash" || market.Plugins[0].Source != "./plugin" {
		t.Errorf("marketplace.json: %+v", market)
	}
	if !strings.Contains(readFile(t, filepath.Join(repoRoot, "plugin/scripts/schemaclash.go")), `const version = "0.1.0"`) {
		t.Error("schemaclash.go version doesn't match plugin.json")
	}
	if !strings.HasPrefix(readFile(t, filepath.Join(repoRoot, skillRel)), "---\nname: finding-otel-schema-url-conflicts\ndescription: ") {
		t.Error("SKILL.md front matter")
	}
	readme := readFile(t, filepath.Join(repoRoot, "README.md"))
	if !strings.Contains(readme, "schemaclash/plugin/scripts@v0.1.0") || !strings.Contains(readme, "schemaclash@schemaclash-marketplace") {
		t.Error("README install lines don't match the version or marketplace name")
	}
	if !strings.Contains(readFile(t, filepath.Join(repoRoot, "CHANGELOG.md")), "## [0.1.0]") {
		t.Error("CHANGELOG.md has no 0.1.0 section")
	}
}
