package remotesrc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/81beastie/logcollector/internal/domain"
)

func setupFiles(t *testing.T) (work string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("FAKEDATA", root)
	anydesk := filepath.Join(root, "AnyDesk")
	os.MkdirAll(anydesk, 0o755)
	os.WriteFile(filepath.Join(anydesk, "ad.trace"), []byte("trace"), 0o644)
	os.WriteFile(filepath.Join(anydesk, "connection_trace.txt"), []byte("conn"), 0o644)
	tv := filepath.Join(root, "TeamViewer")
	os.MkdirAll(tv, 0o755)
	os.WriteFile(filepath.Join(tv, "TeamViewer15_Logfile.log"), []byte("tv"), 0o644)
	return root
}

func registry() []domain.RemoteSource {
	return []domain.RemoteSource{
		{Name: "anydesk", Kind: "remote", Globs: []string{`%FAKEDATA%\AnyDesk\*.trace`, `%FAKEDATA%\AnyDesk\connection_trace.txt`}},
		{Name: "teamviewer", Kind: "remote", Globs: []string{`%FAKEDATA%\TeamViewer\*.log`}},
		{Name: "ammyy", Kind: "remote", Globs: []string{`%FAKEDATA%\Ammyy\*.log`}},
	}
}

func TestCollect_CopiesFoundFiles(t *testing.T) {
	setupFiles(t)
	dest := t.TempDir()
	c := New(registry())

	arts := c.Collect(context.Background(), dest)
	bySrc := map[string]int{}
	for _, a := range arts {
		if !a.Ok() {
			t.Errorf("неожиданная ошибка: %+v", a)
		}
		bySrc[a.Source]++
	}
	if bySrc["anydesk"] != 2 {
		t.Errorf("anydesk: %d артефактов, ожидала 2 (ad.trace + connection_trace)", bySrc["anydesk"])
	}
	if bySrc["teamviewer"] != 1 {
		t.Errorf("teamviewer: %d артефактов, ожидала 1", bySrc["teamviewer"])
	}
	b, err := os.ReadFile(filepath.Join(dest, "anydesk", "ad.trace"))
	if err != nil || string(b) != "trace" {
		t.Errorf("ad.trace не скопирован: %q %v", b, err)
	}
}

func TestCollect_MissingSourceIsSilentSkip(t *testing.T) {
	setupFiles(t)
	arts := New(registry()).Collect(context.Background(), t.TempDir())
	for _, a := range arts {
		if a.Source == "ammyy" {
			t.Errorf("ammyy нет на машине — артефактов быть не должно: %+v", a)
		}
	}
}

func TestCollect_Name(t *testing.T) {
	if New(nil).Name() != "remotefiles" {
		t.Error("Name должен быть remotefiles")
	}
}

func TestCollect_EmptyRegistry(t *testing.T) {
	if arts := New(nil).Collect(context.Background(), t.TempDir()); len(arts) != 0 {
		t.Errorf("пустой реестр — нет артефактов, получено %d", len(arts))
	}
}
