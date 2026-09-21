// Package remotesrc — копирование следов инструментов удалённого доступа
// (AnyDesk, Ammyy, RuDesktop, Radmin, TeamViewer, MeshAgent, Supremo) по реестру источников.
package remotesrc

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/81beastie/logcollector/internal/domain"
	"github.com/81beastie/logcollector/internal/remotefiles"
)

// Collector — копирует файлы следов удалёнки в <dest>/<имя источника>/.
type Collector struct {
	sources []domain.RemoteSource
}

// New — коллектор по реестру источников.
func New(sources []domain.RemoteSource) *Collector { return &Collector{sources: sources} }

// Name — имя источника для отчёта.
func (c *Collector) Name() string { return "remotefiles" }

// Collect — находит файлы по glob-паттернам и копирует их.
// Инструмента может не быть на машине — это не ошибка, просто нет артефактов.
func (c *Collector) Collect(ctx context.Context, dest string) []domain.Artifact {
	var arts []domain.Artifact
	for _, src := range c.sources {
		if err := ctx.Err(); err != nil {
			return arts
		}
		files, err := remotefiles.Resolve(src.Globs)
		if err != nil {
			arts = append(arts, domain.Artifact{Source: src.Name, Err: err.Error()})
			continue
		}
		for _, file := range files {
			art, err := copyOne(file, filepath.Join(dest, src.Name), dest)
			if err != nil {
				arts = append(arts, domain.Artifact{Source: src.Name, Err: err.Error()})
				continue
			}
			arts = append(arts, art)
		}
	}
	return arts
}

// copyOne — копирует файл в подкаталог источника; возвращает артефакт с относительным путём.
func copyOne(file, subdir, dest string) (domain.Artifact, error) {
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		return domain.Artifact{}, fmt.Errorf("каталог %s: %w", subdir, err)
	}
	src, err := os.Open(file)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("чтение %s: %w", file, err)
	}
	defer src.Close()

	target := filepath.Join(subdir, filepath.Base(file))
	dst, err := os.Create(target)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("создание %s: %w", target, err)
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("копирование %s: %w", file, err)
	}
	rel, err := filepath.Rel(dest, target)
	if err != nil {
		rel = target
	}
	return domain.Artifact{Source: filepath.Base(subdir), Path: filepath.ToSlash(rel), Bytes: n}, nil
}
