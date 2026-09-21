package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fakeRoot — подмена корня реестра на тестовый каталог с reg-файлами.
func fakeRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const runReg = `Windows Registry Editor Version 5.00

[HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Run]
"AnyDesk"="C:\\Program Files (x86)\\AnyDesk\\AnyDesk.exe"
"Radmin3"=hex(2):43,00,3a,00
`

const runOnceReg = `Windows Registry Editor Version 5.00

[HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce]
"Cleanup"="cmd /c del temp"
`

const hkcuRunReg = `Windows Registry Editor Version 5.00

[HKEY_CURRENT_USER\SOFTWARE\Microsoft\Windows\CurrentVersion\Run]
"TeamViewer"="C:\\Program Files (x86)\\TeamViewer\\TeamViewer.exe"
`

func TestCollect_ParsesRunKeys(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		`HKLM/SOFTWARE/Microsoft/Windows/CurrentVersion/Run.reg`:     runReg,
		`HKLM/SOFTWARE/Microsoft/Windows/CurrentVersion/RunOnce.reg`: runOnceReg,
		`HKCU/SOFTWARE/Microsoft/Windows/CurrentVersion/Run.reg`:     hkcuRunReg,
	})
	dest := t.TempDir()

	arts := New(root).Collect(context.Background(), dest)
	if len(arts) == 0 {
		t.Fatal("нет артефактов")
	}
	for _, a := range arts {
		if !a.Ok() {
			t.Errorf("ошибка: %+v", a)
		}
	}
	out := filepath.Join(dest, "registry", "run_keys.json")
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("run_keys.json не создан: %v", err)
	}
	var keys []RunKey
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatalf("невалидный JSON: %v", err)
	}
	if len(keys) != 4 {
		t.Errorf("ключей: %d, ожидала 4 (AnyDesk, Radmin3, Cleanup, TeamViewer)", len(keys))
	}
	found := map[string]bool{}
	for _, k := range keys {
		found[k.Name] = true
		if k.Hive == "" || k.Path == "" {
			t.Errorf("ключ %+v без hive/path", k)
		}
	}
	for _, want := range []string{"AnyDesk", "Radmin3", "Cleanup", "TeamViewer"} {
		if !found[want] {
			t.Errorf("ключ %q не найден", want)
		}
	}
}

func TestCollect_MissingHivesAreSilent(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		`HKLM/SOFTWARE/Other.reg`: `[HKEY_LOCAL_MACHINE\SOFTWARE\Other]\n"x"="y"`,
	})
	arts := New(root).Collect(context.Background(), t.TempDir())
	if len(arts) != 0 {
		t.Errorf("Run-ключей нет — артефактов быть не должно: %+v", arts)
	}
}

func TestCollect_Name(t *testing.T) {
	if New(t.TempDir()).Name() != "registry" {
		t.Error("Name = registry")
	}
}

func TestParseRegFile_ExtractsValues(t *testing.T) {
	vals := parseRegContent(`[HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Run]
"A"="1"
"B"=hex(2):43,00
"C"=dword:00000001
`)
	if len(vals) != 3 {
		t.Fatalf("значений: %d, ожидала 3", len(vals))
	}
	if vals[0].Name != "A" || vals[0].Data != "1" {
		t.Errorf("vals[0] = %+v", vals[0])
	}
}
