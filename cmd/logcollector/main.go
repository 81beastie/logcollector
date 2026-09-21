// logcollector — триажный сборщик логов Windows: экспорт EVTX-журналов,
// следы удалённого доступа, упаковка в zip с манифестом.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/81beastie/logcollector/internal/archive"
	"github.com/81beastie/logcollector/internal/collector"
	"github.com/81beastie/logcollector/internal/domain"
	"github.com/81beastie/logcollector/internal/eventlog"
	"github.com/81beastie/logcollector/internal/registry"
	"github.com/81beastie/logcollector/internal/remotesrc"
)

// osStdout — точка подмены в тестах.
var osStdout io.Writer = os.Stdout

func main() {
	os.Exit(mainWithArgs(os.Args[1:]))
}

// mainWithArgs — парсинг флагов и запуск; код возврата для тестов.
func mainWithArgs(args []string) int {
	fs := flag.NewFlagSet("logcollector", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cfg config
	registerFlags(fs, &cfg)
	fs.Usage = func() { printUsage(osStdout) }
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		printUsage(osStdout)
		return 2
	}
	if cfg.showVersion {
		fmt.Fprintf(osStdout, "logcollector %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	if rest := fs.Args(); len(rest) > 0 {
		cfg.out = rest[0]
	}
	if err := run(cfg); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}

type config struct {
	out         string
	noEventlog  bool
	noRemote    bool
	noRegistry  bool
	showVersion bool
}

func registerFlags(fs *flag.FlagSet, cfg *config) {
	fs.StringVar(&cfg.out, "out", ".", "каталог для итогового zip")
	fs.BoolVar(&cfg.noEventlog, "no-eventlog", false, "не экспортировать журналы событий")
	fs.BoolVar(&cfg.noRemote, "no-remote", false, "не собирать следы удалённого доступа и антивирусов")
	fs.BoolVar(&cfg.noRegistry, "no-registry", false, "не собирать Run-ключи автозапуска")
	fs.BoolVar(&cfg.showVersion, "version", false, "показать версию и выйти")
}

// run — сбор бандла и упаковка.
func run(cfg config) error {
	if cfg.out == "" {
		return fmt.Errorf("пустой каталог назначения")
	}
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return fmt.Errorf("каталог назначения: %w", err)
	}

	host, err := os.Hostname()
	if err != nil {
		host = "UNKNOWN"
	}

	var sources []collector.Collector
	if !cfg.noEventlog {
		sources = append(sources, eventlog.New("wevtutil", domain.EventLogNames))
	}
	if !cfg.noRemote {
		sources = append(sources, remotesrc.New(domain.AllFileSources()))
	}
	if !cfg.noRegistry {
		sources = append(sources, registry.New(""))
	}
	if len(sources) == 0 {
		return fmt.Errorf("все источники отключены флагами")
	}

	work, err := os.MkdirTemp("", "logcollector-*")
	if err != nil {
		return fmt.Errorf("рабочий каталог: %w", err)
	}
	defer os.RemoveAll(work)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	fmt.Fprintln(osStdout, "сбор логов на", host)
	bundle, err := collector.NewRunner(sources).Run(ctx, work, host)
	if err != nil {
		return err
	}

	out := archive.OutPath(cfg.out, host)
	if err := archive.Zip(filepath.Join(work, host), out, bundle.Errors()); err != nil {
		return fmt.Errorf("упаковка: %w", err)
	}
	bundle.OutPath = out

	fmt.Fprint(osStdout, bundle.Report())
	return nil
}

// printUsage — справка на русском.
func printUsage(w io.Writer) {
	fmt.Fprint(w, `logcollector `+version+` — триажный сборщик логов Windows в один zip.

Использование:
  logcollector [флаги] [каталог-назначения]     итог: <каталог>\<хост>.zip

Собирает:
  - EVTX-экспорт журналов (Security, System, RDP, PowerShell, WinRM, ...)
  - следы удалённого доступа (AnyDesk, Ammyy, RuDesktop, Radmin, TeamViewer, MeshAgent,
    Supremo, AeroAdmin, UltraViewer, DWService, LogMeIn)
  - логи антивирусов (Kaspersky, DrWeb, AVG, Avast, ESET, Windows Defender)
  - Run-ключи автозапуска (HKLM/HKCU Run, RunOnce — персистентность вредоноса)

Флаги:
  -out каталог     каталог для итогового zip (по умолчанию текущий)
  -no-eventlog     не экспортировать журналы событий
  -no-remote       не собирать следы удалённого доступа
  -version         версия и выход
  -h, -help        эта справка

Примечание: для журнала Security нужны права администратора —
запускай elevated (от имени администратора).

Примеры:
  logcollector D:\triage
  logcollector -out \\\\fileserver\triage
`)
}
