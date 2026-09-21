package collector

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/81beastie/logcollector/internal/domain"
)

// fakeCollector — тестовый источник: создаёт файлы в dest и возвращает артефакты.
type fakeCollector struct {
	name    string
	files   map[string]int64 // имя файла внутри dest -> размер
	artErr  string           // если задано — артефакт с ошибкой
	collect func(dest string) error
}

func (f *fakeCollector) Name() string { return f.name }

func (f *fakeCollector) Collect(_ context.Context, dest string) []domain.Artifact {
	if f.collect != nil {
		if err := f.collect(dest); err != nil {
			return []domain.Artifact{{Source: f.name, Err: err.Error()}}
		}
	}
	arts := make([]domain.Artifact, 0, len(f.files))
	for name, size := range f.files {
		if err := os.WriteFile(filepath.Join(dest, name), make([]byte, size), 0o644); err != nil {
			arts = append(arts, domain.Artifact{Source: f.name, Err: err.Error()})
			continue
		}
		art := domain.Artifact{Source: f.name, Path: name, Bytes: size}
		if f.artErr != "" {
			art.Err = f.artErr
		}
		arts = append(arts, art)
	}
	return arts
}

func TestRunner_CollectsFromAllSources(t *testing.T) {
	work := t.TempDir()
	r := NewRunner([]Collector{
		&fakeCollector{name: "eventlog", files: map[string]int64{"Security.evtx": 100, "System.evtx": 50}},
		&fakeCollector{name: "anydesk", files: map[string]int64{"ad.trace": 10}},
	})

	bundle, err := r.Run(context.Background(), work, "HOST1")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Host != "HOST1" {
		t.Errorf("Host = %q, ожидала HOST1", bundle.Host)
	}
	ok, failed, total := bundle.Counts()
	if ok != 3 || failed != 0 || total != 160 {
		t.Errorf("Counts = %d/%d/%d, ожидала 3/0/160", ok, failed, total)
	}
	for _, name := range []string{"Security.evtx", "System.evtx", "ad.trace"} {
		if _, err := os.Stat(filepath.Join(work, "HOST1", name)); err != nil {
			t.Errorf("файл %s не в рабочем каталоге: %v", name, err)
		}
	}
}

func TestRunner_FailureOfOneSourceDoesNotStopOthers(t *testing.T) {
	work := t.TempDir()
	r := NewRunner([]Collector{
		&fakeCollector{name: "broken", collect: func(string) error { return errors.New("нет доступа") }},
		&fakeCollector{name: "good", files: map[string]int64{"x.log": 5}},
	})

	bundle, err := r.Run(context.Background(), work, "HOST1")
	if err != nil {
		t.Fatalf("ошибка одного источника не должна валить весь сбор: %v", err)
	}
	ok, failed, _ := bundle.Counts()
	if ok != 1 || failed != 1 {
		t.Errorf("Counts = %d/%d, ожидала 1 собран / 1 ошибка", ok, failed)
	}
}

func TestRunner_EmptyRegistryGivesEmptyBundle(t *testing.T) {
	bundle, err := NewRunner(nil).Run(context.Background(), t.TempDir(), "HOST1")
	if err != nil {
		t.Fatal(err)
	}
	if ok, failed, _ := bundle.Counts(); ok != 0 || failed != 0 {
		t.Error("без источников — пустой бандл без ошибок")
	}
}

func TestRunner_ContextCancelAborts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := NewRunner([]Collector{&fakeCollector{name: "slow", files: map[string]int64{"x": 1}}})
	if _, err := r.Run(ctx, t.TempDir(), "HOST1"); err == nil {
		t.Error("отменённый контекст должен прерывать сбор")
	}
}

func TestRunner_HostDirPerHost(t *testing.T) {
	work := t.TempDir()
	r := NewRunner([]Collector{&fakeCollector{name: "good", files: map[string]int64{"x.log": 1}}})
	if _, err := r.Run(context.Background(), work, "ALPHA"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(context.Background(), work, "BETA"); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"ALPHA", "BETA"} {
		if _, err := os.Stat(filepath.Join(work, h)); err != nil {
			t.Errorf("каталог хоста %s не создан", h)
		}
	}
}
