package remotefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpand_EnvVars(t *testing.T) {
	t.Setenv("ProgramData", `C:\ProgramData`)
	got := Expand(`%ProgramData%\AnyDesk\ad.trace`)
	if got != `C:\ProgramData\AnyDesk\ad.trace` {
		t.Errorf("Expand = %q", got)
	}
}

func TestExpand_MultipleVars(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	t.Setenv("TEMP", `C:\Temp`)
	got := Expand(`%SystemRoot%\x\%TEMP%\y`)
	if got != `C:\Windows\x\C:\Temp\y` {
		t.Errorf("Expand = %q", got)
	}
}

func TestExpand_UnknownVarStays(t *testing.T) {
	got := Expand(`%NoSuchVar%\file.log`)
	if got != `%NoSuchVar%\file.log` {
		t.Errorf("неизвестная переменная должна оставаться как есть: %q", got)
	}
}

func TestResolve_FindsFilesByGlob(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FAKEROOT", dir)
	for _, name := range []string{"a.log", "b.log", "skip.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Resolve([]string{`%FAKEROOT%\*.log`})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("найдено %d файлов, ожидала 2 (a.log, b.log): %v", len(got), got)
	}
}

func TestResolve_MissingGlobIsNotError(t *testing.T) {
	got, err := Resolve([]string{filepath.Join(t.TempDir(), "нет", "*.log")})
	if err != nil {
		t.Fatalf("несуществующий путь — пустой результат, не ошибка: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ожидала пусто, получено %v", got)
	}
}

func TestResolve_UniqueFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve([]string{`%TEMP%`, filepath.Join(dir, "*.log"), filepath.Join(dir, "*.log")})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, p := range got {
		if seen[p] {
			t.Errorf("дубль %q", p)
		}
		seen[p] = true
	}
}
