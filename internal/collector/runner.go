package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/81beastie/logcollector/internal/domain"
)

// Runner — оркестратор: прогоняет реестр источников и собирает бандл.
type Runner struct {
	sources []Collector
}

// NewRunner — сборщик по списку источников.
func NewRunner(sources []Collector) *Runner { return &Runner{sources: sources} }

// Run — собирает артефакты всех источников в <work>/<host>.
// Ошибка отдельного источника не прерывает сбор — уходит в Artifact.Err.
func (r *Runner) Run(ctx context.Context, work, host string) (*domain.Bundle, error) {
	if host == "" {
		return nil, fmt.Errorf("пустое имя хоста")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("сбор отменён: %w", err)
	}

	dest := filepath.Join(work, host)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("рабочий каталог %s: %w", dest, err)
	}

	bundle := &domain.Bundle{Host: host, Artifacts: []domain.Artifact{}}
	for _, src := range r.sources {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("сбор отменён: %w", err)
		}
		bundle.Artifacts = append(bundle.Artifacts, src.Collect(ctx, dest)...)
	}
	return bundle, nil
}
