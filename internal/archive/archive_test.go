package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/81beastie/logcollector/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

// prepare — создаёт рабочую структуру хоста с файлами.
func prepare(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	dir := filepath.Join(work, "HOST1")
	for name, content := range map[string]string{
		"eventlog/Security.evtx": "evtx-bytes",
		"anydesk/ad.trace":       "trace-bytes",
	} {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readZip(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}
	return files
}

func TestZip_CreatesArchiveWithHostPrefix(t *testing.T) {
	dir := prepare(t)
	out := filepath.Join(t.TempDir(), "bundle.zip")

	if err := Zip(dir, out, nil); err != nil {
		t.Fatal(err)
	}
	files := readZip(t, out)
	for _, want := range []string{"HOST1/eventlog/Security.evtx", "HOST1/anydesk/ad.trace"} {
		if _, ok := files[want]; !ok {
			t.Errorf("в архиве нет %q; есть: %v", want, keys(files))
		}
	}
	if files["HOST1/anydesk/ad.trace"] != "trace-bytes" {
		t.Error("содержимое файла в архиве искажено")
	}
}

func TestZip_ManifestInside(t *testing.T) {
	dir := prepare(t)
	out := filepath.Join(t.TempDir(), "bundle.zip")

	if err := Zip(dir, out, nil); err != nil {
		t.Fatal(err)
	}
	files := readZip(t, out)
	m, ok := files["HOST1/manifest.json"]
	if !ok {
		t.Fatalf("manifest.json нет в архиве; есть: %v", keys(files))
	}
	for _, want := range []string{`"host": "HOST1"`, `"started_at"`, `"artifacts"`} {
		if !contains(m, want) {
			t.Errorf("manifest не содержит %q:\n%s", want, m)
		}
	}
}

func TestZip_EmptyDirStillZips(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "empty.zip")
	if err := Zip(dir, out, nil); err != nil {
		t.Fatalf("пустой каталог должен зиповаться без ошибки: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Error("итоговый zip не создан")
	}
}

func TestZip_MissingSourceDirIsError(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.zip")
	if err := Zip(filepath.Join(t.TempDir(), "нет"), out, nil); err == nil {
		t.Error("отсутствующий каталог — ошибка")
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestZip_WithErrors_FixesFailuresInManifest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "h")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "a.log"), []byte("x"), 0o644)
	out := filepath.Join(t.TempDir(), "h.zip")

	errs := []domain.CollectError{
		{Source: "eventlog:Security", Err: "отказано в доступе"},
		{Source: "eventlog:Sysmon", Err: "журнал не существует"},
	}
	if err := Zip(dir, out, errs); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	for _, f := range zr.File {
		if f.Name == "h/manifest.json" {
			r, _ := f.Open()
			raw, _ := io.ReadAll(r)
			r.Close()
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("невалидный манифест: %v", err)
			}
		}
	}
	if len(m.Errors) != 2 {
		t.Fatalf("errors в манифесте: %d, ожидала 2", len(m.Errors))
	}
	if m.Errors[0].Source != "eventlog:Security" || m.Errors[0].Err != "отказано в доступе" {
		t.Errorf("errors[0] = %+v", m.Errors[0])
	}
	if m.Errors[1].Source != "eventlog:Sysmon" {
		t.Errorf("errors[1] = %+v", m.Errors[1])
	}
}

func TestZip_NoErrors_OmitsEmptySection(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "h")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "a.log"), []byte("x"), 0o644)
	out := filepath.Join(t.TempDir(), "h.zip")

	if err := Zip(dir, out, nil); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	zr, _ := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	for _, f := range zr.File {
		if f.Name == "h/manifest.json" {
			r, _ := f.Open()
			raw, _ := io.ReadAll(r)
			r.Close()
			if strings.Contains(string(raw), `"errors"`) {
				t.Error("пустая секция errors не должна попадать в манифест")
			}
		}
	}
}
