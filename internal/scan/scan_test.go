package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestScanDirs_Basic(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import "github.com/im-wmkong/errkind"

var (
	A = errkind.Define(10001, "user_not_found")
	B = errkind.Define(10002, "user_locked")
)
`)
	// 不导入 errkind 的文件应被忽略
	writeGo(t, filepath.Join(dir, "b.go"), `package x

func F() {}
`)
	// _test.go 应被跳过
	writeGo(t, filepath.Join(dir, "c_test.go"), `package x

import "github.com/im-wmkong/errkind"

var Z = errkind.Define(99999, "in_test")
`)

	defs, errs := ScanDirs([]string{dir})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Code < defs[j].Code })

	if len(defs) != 2 {
		t.Fatalf("got %d defs, want 2: %+v", len(defs), defs)
	}
	if defs[0].Code != 10001 || defs[0].Name != "user_not_found" {
		t.Errorf("def[0] = %+v", defs[0])
	}
	if defs[1].Code != 10002 || defs[1].Name != "user_locked" {
		t.Errorf("def[1] = %+v", defs[1])
	}
}

func TestScanDirs_AliasImport(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import ek "github.com/im-wmkong/errkind"

var A = ek.Define(1, "n")
`)
	defs, errs := ScanDirs([]string{dir})
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(defs) != 1 || defs[0].Code != 1 || defs[0].Name != "n" {
		t.Fatalf("defs = %+v", defs)
	}
}

func TestScanDirs_SkipDotImport(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import . "github.com/im-wmkong/errkind"

var A = Define(1, "n")
`)
	defs, _ := ScanDirs([]string{dir})
	if len(defs) != 0 {
		t.Fatalf("dot-import should be ignored, got %+v", defs)
	}
}

func TestScanDirs_NonLiteralIgnored(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, filepath.Join(dir, "a.go"), `package x

import "github.com/im-wmkong/errkind"

const C = 1
var name = "n"
var A = errkind.Define(C, name)
`)
	defs, errs := ScanDirs([]string{dir})
	if len(defs) != 0 || len(errs) != 1 {
		t.Fatalf("non-literal args must be diagnosed, got defs=%+v errors=%v", defs, errs)
	}
}

func TestDefineArgumentCount(t *testing.T) {
	for _, args := range []string{`1`, `1, "x", nil`} {
		dir := t.TempDir()
		writeGo(t, filepath.Join(dir, "a.go"), `package x; import "github.com/im-wmkong/errkind"; var X = errkind.Define(`+args+`)`)
		defs, errs := ScanDirs([]string{dir})
		if len(defs) != 0 || len(errs) != 1 || !strings.Contains(errs[0].Error(), "exactly two") {
			t.Fatalf("defs=%v errors=%v", defs, errs)
		}
	}
}

func TestScanExcludedAndOverlappingPaths(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.go")
	writeGo(t, valid, `package x; import "github.com/im-wmkong/errkind"; var X = errkind.Define(1, "x")`)
	writeGo(t, filepath.Join(dir, "broken.go"), "not Go source")
	defs, errs := ScanDirs([]string{dir, valid}, "broken.go")
	if len(defs) != 1 || len(errs) != 0 {
		t.Fatalf("excluded or duplicate paths: defs=%v errors=%v", defs, errs)
	}
	if _, errs := ScanDirs([]string{dir}, "["); len(errs) != 1 {
		t.Fatal("invalid exclude must fail")
	}
}

func TestCLIExitCodes(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	fixture := t.TempDir()
	valid := filepath.Join(fixture, "valid.go")
	broken := filepath.Join(fixture, "broken.go")
	nonliteral := filepath.Join(fixture, "nonliteral.go")
	writeGo(t, valid, `package x; import "github.com/im-wmkong/errkind"; var X = errkind.Define(1, "x")`)
	writeGo(t, broken, "not Go source")
	writeGo(t, nonliteral, `package x; import "github.com/im-wmkong/errkind"; const C = 1; var X = errkind.Define(C, "x")`)
	for _, tool := range []string{"errkind", "errkindlint"} {
		bin := filepath.Join(binDir, tool)
		build := exec.Command("go", "build", "-o", bin, "./cmd/"+tool)
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, out)
		}
		for _, path := range []string{valid, broken, nonliteral, filepath.Join(fixture, "missing")} {
			args := []string{path}
			if tool == "errkind" {
				args = []string{"doc", "-format=json", path}
			}
			out, err := exec.Command(bin, args...).CombinedOutput()
			if (err == nil) != (path == valid) {
				t.Fatalf("%s %s: %v %s", tool, path, err, out)
			}
		}
		if tool == "errkindlint" {
			out, err := exec.Command(bin, "-exclude=broken.go", "-exclude=nonliteral.go", fixture).CombinedOutput()
			if err != nil {
				t.Fatalf("excluded files parsed: %v %s", err, out)
			}
		} else {
			output := filepath.Join(binDir, "output.json")
			if err := os.WriteFile(output, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"doc", "-format=invalid", "-o", output, valid}, {"doc", "-o", output, broken}} {
				if err := exec.Command(bin, args...).Run(); err == nil {
					t.Fatal("invalid command succeeded")
				}
				data, err := os.ReadFile(output)
				if err != nil || strings.TrimSpace(string(data)) != "keep" {
					t.Fatal("failed command overwrote output")
				}
			}
		}
	}
}

func writeGo(t *testing.T, path, content string) {
	t.Helper()
	if err := writeFile(path, []byte(content)); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
