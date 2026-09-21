// Package eventlog — экспорт журналов Windows в .evtx через wevtutil.
package eventlog

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/81beastie/logcollector/internal/domain"
	"github.com/81beastie/logcollector/internal/wincp"
)

// Collector — экспортёр журналов событий через wevtutil e <log> /l:<file>.
type Collector struct {
	wevtutil string // путь к wevtutil (подменяется в тестах)
	logs     []string
}

// New — экспортёр для списка журналов.
func New(wevtutil string, logs []string) *Collector {
	return &Collector{wevtutil: wevtutil, logs: logs}
}

// Name — имя источника для отчёта.
func (c *Collector) Name() string { return "eventlog" }

// Collect — экспортирует каждый журнал в <dest>/eventlog/<имя>.evtx.
// Отказ одного журнала не останавливает остальные — уходит в Artifact.Err.
func (c *Collector) Collect(ctx context.Context, dest string) []domain.Artifact {
	arts := make([]domain.Artifact, 0, len(c.logs))
	sub := filepath.Join(dest, "eventlog")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return append(arts, domain.Artifact{Source: c.Name(), Err: err.Error()})
	}
	for _, log := range c.logs {
		rel := filepath.Join("eventlog", flatName(log)+".evtx")
		art := domain.Artifact{Source: "eventlog:" + log, Path: rel}
		if err := c.export(ctx, filepath.Join(dest, rel), log); err != nil {
			art.Err = err.Error()
			arts = append(arts, art)
			continue
		}
		if fi, err := os.Stat(filepath.Join(dest, rel)); err == nil {
			art.Bytes = fi.Size()
		}
		arts = append(arts, art)
	}
	return arts
}

// export — один вызов wevtutil epl <журнал> <файл> /ow (epl = export-log).
func (c *Collector) export(ctx context.Context, out, log string) error {
	cmd := exec.CommandContext(ctx, c.wevtutil, "epl", log, out, "/ow")
	if b, err := cmd.CombinedOutput(); err != nil {
		// wevtutil пишет в OEM-кодировке (CP866) — декодируем для читаемого отчёта
		return fmt.Errorf("%s: %s", log, strings.TrimSpace(wincp.Decode(b)))
	}
	return nil
}

// flatName — имя файла из имени журнала: слэши и недопустимые символы → '_'.
func flatName(log string) string {
	return strings.NewReplacer("/", "_", `\`, "_", ":", "_").Replace(log)
}
