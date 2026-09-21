// Package archive — упаковка рабочей структуры хоста в zip с манифестом.
package archive

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/81beastie/logcollector/internal/domain"
)

// Manifest — паспорт бандла: что, когда и на каком хосте собрано.
type Manifest struct {
	Host      string                `json:"host"`
	StartedAt string                `json:"started_at"`
	Tool      string                `json:"tool"`
	Artifacts []Entry               `json:"artifacts"`
	Errors    []domain.CollectError `json:"errors,omitempty"` // отказы источников: пусто — секции нет
}

// Entry — файл в манифесте.
type Entry struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// Zip — упаковывает dir (каталог хоста) в out; внутри архива пути с префиксом <basename(dir)>/.
// manifest.json записывается в корень секции хоста; errors — зафиксированные отказы
// источников (nil/пусто = секция не пишется).
func Zip(dir, out string, errors []domain.CollectError) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("каталог сбора: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s — не каталог", dir)
	}
	host := filepath.Base(dir)

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("итоговый файл: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	m := Manifest{Host: host, StartedAt: time.Now().UTC().Format(time.RFC3339), Tool: "logcollector", Artifacts: []Entry{}, Errors: errors}

	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		zipName := filepath.ToSlash(filepath.Join(host, rel))
		if err := addFile(zw, path, zipName); err != nil {
			return err
		}
		m.Artifacts = append(m.Artifacts, Entry{Path: zipName, Bytes: fi.Size()})
		return nil
	})
	if err != nil {
		return fmt.Errorf("обход каталога сбора: %w", err)
	}

	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := addBytes(zw, filepath.ToSlash(filepath.Join(host, "manifest.json")), mb); err != nil {
		return err
	}
	return zw.Close()
}

func addFile(zw *zip.Writer, path, name string) error {
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("чтение %s: %w", path, err)
	}
	defer src.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

func addBytes(zw *zip.Writer, name string, b []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// OutPath — итоговое имя архива для хоста и каталога назначения.
func OutPath(outDir, host string) string {
	return filepath.Join(outDir, strings.ReplaceAll(host, " ", "_")+".zip")
}
