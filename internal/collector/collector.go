// Package collector — интерфейс источника артефактов и оркестратор сбора.
package collector

import (
	"context"

	"github.com/81beastie/logcollector/internal/domain"
)

// Collector — источник артефактов: экспорт журналов, копирование следов удалёнки и т.п.
// Реализации лежат в своих пакетах; ошибки возвращаются как данные в Artifact.Err.
type Collector interface {
	// Name — имя источника для отчёта (eventlog, anydesk, ...).
	Name() string
	// Collect — собирает артефакты в dest (каталог внутри рабочей папки).
	Collect(ctx context.Context, dest string) []domain.Artifact
}
