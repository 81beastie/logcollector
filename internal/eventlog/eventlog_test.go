package eventlog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeWevtutil — подменяемая команда экспорта: epl <журнал> <файл> /ow.
func fakeWevtutil(t *testing.T, marker string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "wevtutil")
	script := "#!/bin/sh\n[ $# -lt 3 ] && exit 3\ncmd=\"$1\"; log=\"$2\"; out=\"$3\"\n[ \"$cmd\" != \"epl\" ] && exit 3\nprintf '" + marker + " %s' \"$log\" > \"$out\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCollect_ExportsEachLog(t *testing.T) {
	dir := t.TempDir()
	c := New(fakeWevtutil(t, "EVTX"), []string{"Security", "System"})

	arts := c.Collect(context.Background(), dir)
	if len(arts) != 2 {
		t.Fatalf("артефактов: %d, ожидала 2", len(arts))
	}
	for _, a := range arts {
		if !a.Ok() {
			t.Fatalf("артефакт с ошибкой: %+v", a)
		}
		if a.Source != "eventlog:Security" && a.Source != "eventlog:System" {
			t.Errorf("источник = %q", a.Source)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "eventlog", "Security.evtx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "EVTX Security") {
		t.Errorf("файл Security.evtx: %q", b)
	}
}

func TestCollect_UnknownLogBecomesError(t *testing.T) {
	dir := t.TempDir()
	// скрипт завершается кодом 3 при пустом логе — эмулируем отказ wevtutil
	bad := filepath.Join(t.TempDir(), "wevtutil")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := New(bad, []string{"Security"})

	arts := c.Collect(context.Background(), dir)
	if len(arts) != 1 || arts[0].Ok() {
		t.Fatalf("ожидала артефакт с ошибкой, получено: %+v", arts)
	}
	if !strings.Contains(arts[0].Err, "Security") {
		t.Errorf("в ошибке должно быть имя журнала: %+v", arts[0])
	}
}

func TestCollect_NameAndDestSubdir(t *testing.T) {
	c := New(fakeWevtutil(t, "X"), []string{"Security"})
	if c.Name() != "eventlog" {
		t.Errorf("Name = %q, ожидала eventlog", c.Name())
	}
	arts := c.Collect(context.Background(), t.TempDir())
	if arts[0].Path != filepath.Join("eventlog", "Security.evtx") {
		t.Errorf("Path = %q, ожидала eventlog/Security.evtx", arts[0].Path)
	}
}

func TestCollect_SlashInLogNameFlattened(t *testing.T) {
	dir := t.TempDir()
	c := New(fakeWevtutil(t, "E"), []string{"Microsoft-Windows-PowerShell/Operational"})

	arts := c.Collect(context.Background(), dir)
	if !arts[0].Ok() {
		t.Fatalf("ошибка: %+v", arts[0])
	}
	if strings.ContainsRune(arts[0].Path, '/') && filepath.Separator == '\\' {
		t.Errorf("слэш в имени файла %q", arts[0].Path)
	}
	if _, err := os.Stat(filepath.Join(dir, arts[0].Path)); err != nil {
		t.Errorf("файл не создан: %v", err)
	}
}

func TestFlatName(t *testing.T) {
	cases := map[string]string{
		"Security": "Security",
		"Microsoft-Windows-PowerShell/Operational":                               "Microsoft-Windows-PowerShell_Operational",
		"Microsoft-Windows-TerminalServices-RemoteConnectionManager/Operational": "Microsoft-Windows-TerminalServices-RemoteConnectionManager_Operational",
	}
	for in, want := range cases {
		if got := flatName(in); got != want {
			t.Errorf("flatName(%q) = %q, ожидала %q", in, got, want)
		}
	}
}
