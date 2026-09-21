package main

import (
	"archive/zip"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainWithArgs_Version(t *testing.T) {
	out := capture(t, func() int { return mainWithArgs([]string{"-version"}) })
	if !strings.Contains(out, "logcollector ") {
		t.Errorf("вывод -version: %q", out)
	}
}

func TestMainWithArgs_Help(t *testing.T) {
	out := capture(t, func() int { return mainWithArgs([]string{"-h"}) })
	for _, want := range []string{"Использование:", "Собирает:", "-no-eventlog", "Примеры:"} {
		if !strings.Contains(out, want) {
			t.Errorf("справка не содержит %q", want)
		}
	}
}

func TestMainWithArgs_BadFlag(t *testing.T) {
	code := captureCode(t, func() int { return mainWithArgs([]string{"--нет"}) })
	if code != 2 {
		t.Errorf("код = %d, ожидала 2", code)
	}
}

func TestRun_NoEventlogNoRemoteStillRunsRegistry(t *testing.T) {
	out := t.TempDir()
	if err := run(config{out: out, noEventlog: true, noRemote: true}); err != nil {
		t.Fatalf("registry остаётся источником — ошибки быть не должно: %v", err)
	}
}

func TestRun_AllSourcesOffIsError(t *testing.T) {
	if err := run(config{out: t.TempDir(), noEventlog: true, noRemote: true, noRegistry: true}); err == nil {
		t.Error("все источники выключены — должна быть ошибка")
	}
}

func TestRun_ProducesZipWithManifest(t *testing.T) {
	out := t.TempDir()
	if err := run(config{out: out, noEventlog: true}); err != nil {
		t.Fatal(err)
	}
	host, _ := os.Hostname()
	zpath := filepath.Join(out, host+".zip")
	if _, err := os.Stat(zpath); err != nil {
		t.Fatalf("zip не создан: %v", err)
	}
	zr, err := zip.OpenReader(zpath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names[filepath.ToSlash(filepath.Join(host, "manifest.json"))] {
		t.Errorf("manifest.json нет в архиве: %v", names)
	}
}

func TestRun_PositionalArgIsOut(t *testing.T) {
	dir := t.TempDir()
	code := captureCode(t, func() int { return mainWithArgs([]string{"-no-eventlog", "-no-remote", "-no-registry", dir}) })
	if code != 1 {
		t.Errorf("все источники выключены — код 1, получено %d", code)
	}
}

func TestRegisterFlags_AllFlags(t *testing.T) {
	fs := newFlagSetForTest()
	var cfg config
	registerFlags(fs, &cfg)
	if err := fs.Parse([]string{"-out", "/x", "-no-eventlog", "-no-remote", "-no-registry", "-version"}); err != nil {
		t.Fatal(err)
	}
	if cfg.out != "/x" || !cfg.noEventlog || !cfg.noRemote || !cfg.noRegistry || !cfg.showVersion {
		t.Errorf("флаги не применились: %+v", cfg)
	}
}

func capture(t *testing.T, fn func() int) string {
	t.Helper()
	return captureFull(t, fn).out
}

func captureCode(t *testing.T, fn func() int) int {
	t.Helper()
	return captureFull(t, fn).code
}

func captureFull(t *testing.T, fn func() int) (r struct {
	out  string
	code int
}) {
	t.Helper()
	old := osStdout
	path := filepath.Join(t.TempDir(), "out.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	osStdout = f
	code := fn()
	f.Close()
	osStdout = old
	b, _ := os.ReadFile(path)
	r.out, r.code = string(b), code
	return
}

func newFlagSetForTest() *flag.FlagSet { return flag.NewFlagSet("t", flag.ContinueOnError) }
