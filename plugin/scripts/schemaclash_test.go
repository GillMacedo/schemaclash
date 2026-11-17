package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const goodList = `{"ImportPath":"a","Name":"a"}
{"ImportPath":"b","Name":"main","Imports":["a"]}
`

func TestDecodeList(t *testing.T) {
	pkgs, err := decodeList(strings.NewReader(goodList))
	if err != nil || len(pkgs) != 2 {
		t.Fatalf("got %d packages, %v", len(pkgs), err)
	}
}

func TestDecodeListRefuses(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"white space":   "\n \n",
		"garbage":       "\x00\x01 not json",
		"text":          "go: cannot find main module\n",
		"cut":           goodList[:len(goodList)-10],
		"no path":       `{"Name":"a"}`,
		"package error": `{"ImportPath":"a","Error":{"Err":"no Go files"}}`,
		"deps error":    `{"ImportPath":"a","DepsErrors":[{"Err":"cannot find package"}]}`,
		"incomplete":    `{"ImportPath":"a","Incomplete":true}`,
		"array":         `[{"ImportPath":"a"}]`,
	}
	for name, in := range cases {
		_, err := decodeList(strings.NewReader(in))
		var u untrusted
		if !errors.As(err, &u) {
			t.Errorf("%s: want a refusal, got %v", name, err)
		}
	}
	// Every cut of a good stream is refused, except a cut right after a
	// whole object, which can't be told apart from the end of the stream.
	// schemaclash reads the stream only after go list exits 0, so a cut
	// one would need go list itself to stop early and still succeed.
	first := strings.SplitAfter(goodList, "\n")[0]
	for n := 0; n < len(goodList); n++ {
		_, err := decodeList(strings.NewReader(goodList[:n]))
		cut := strings.TrimSpace(goodList[:n])
		if err == nil && cut != strings.TrimSpace(first) && cut != strings.TrimSpace(goodList) {
			t.Errorf("cut at %d accepted: %q", n, goodList[:n])
		}
	}
}

func TestFirstLines(t *testing.T) {
	in := "go: downloading a v1\ngo: downloading b v2\nmain.go:1: bad\nsecond\n"
	if got := firstLines(in, 1); got != "main.go:1: bad; (1 more lines)" {
		t.Errorf("got %q", got)
	}
	if got := firstLines("go: downloading a v1\n", 3); got != "(no message)" {
		t.Errorf("got %q", got)
	}
}

func TestVer(t *testing.T) {
	if ver("https://opentelemetry.io/schemas/1.26.0") != "1.26.0" {
		t.Error("ver of a schema URL")
	}
	if ver("https://example.com/s") != "https://example.com/s" {
		t.Error("ver of another URL should be the URL")
	}
}

func TestFixText(t *testing.T) {
	sdk := sdkInfo{Module: "go.opentelemetry.io/otel/sdk v1.32.0", SchemaURL: "https://opentelemetry.io/schemas/1.26.0"}
	ins := []input{
		{URL: sdk.SchemaURL, Origin: origin{URL: sdk.SchemaURL, inDep: true, mod: "go.opentelemetry.io/otel/sdk v1.32.0"}},
		{URL: "https://opentelemetry.io/schemas/1.24.0", Origin: origin{URL: "https://opentelemetry.io/schemas/1.24.0", What: "semconv.SchemaURL (go.opentelemetry.io/otel/semconv/v1.24.0)", Place: place{File: "t.go", Line: 3}}},
		{URL: "https://opentelemetry.io/schemas/1.21.0", Origin: origin{URL: "https://opentelemetry.io/schemas/1.21.0", inDep: true, mod: "example.com/det v1.0.0", Place: place{File: "example.com/det@v1.0.0/d.go", Line: 9}}},
	}
	got := fixText(sdk, ins)
	for _, want := range []string{
		"in t.go, import go.opentelemetry.io/otel/semconv/v1.26.0 instead of go.opentelemetry.io/otel/semconv/v1.24.0",
		"example.com/det v1.0.0 uses schema 1.21.0",
		"Schema 1.26.0 is the one go.opentelemetry.io/otel/sdk v1.32.0 uses",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fix text lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "sdk v1.32.0 uses schema") {
		t.Errorf("fix text tells the user to change the SDK's own schema:\n%s", got)
	}
	two := fixText(sdk, ins[1:])
	if !strings.HasPrefix(two, "use one semconv version") {
		t.Errorf("without the SDK's schema the fix should ask for one version:\n%s", two)
	}
}

func TestFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "yaml"},
		{"--fail-on", "warn"},
		{"--unknown"},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 2 || out.Len() != 0 {
			t.Errorf("%v: exit %d, stdout %q", args, code, out.String())
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"--version"}, &out, &errb); code != 0 || out.String() != "schemaclash "+version+"\n" {
		t.Errorf("--version: exit %d, %q", code, out.String())
	}
}

func TestMissingDir(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--fail-on", "never", "--dir", t.TempDir() + "/missing"}, &out, &errb)
	if code != 2 || out.Len() != 0 || !strings.Contains(errb.String(), "not a directory") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
}
