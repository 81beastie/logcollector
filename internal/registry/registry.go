// Package registry — сбор Run-ключей автозапуска (персистентность вредоноса):
// экспорт веток через reg export и разбор значений в JSON.
package registry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/81beastie/logcollector/internal/domain"
	"github.com/81beastie/logcollector/internal/wincp"
)

// runKeyPaths — ветки автозапуска (HKLM/HKCU, Run и RunOnce).
var runKeyPaths = []struct {
	hive string // HKLM | HKCU
	key  string // SOFTWARE\Microsoft\Windows\CurrentVersion\Run
}{
	{"HKLM", `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`},
	{"HKLM", `SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce`},
	{"HKLM", `SOFTWARE\Wow6432Node\Microsoft\Windows\CurrentVersion\Run`},
	{"HKCU", `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`},
	{"HKCU", `SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce`},
}

// networkListPaths — история сетей Wi-Fi: профили и сигнатуры точек доступа.
var networkListPaths = []struct {
	hive string
	key  string
}{
	{"HKLM", `SOFTWARE\Microsoft\Windows NT\CurrentVersion\NetworkList\Profiles`},
	{"HKLM", `SOFTWARE\Microsoft\Windows NT\CurrentVersion\NetworkList\Signatures`},
}

// RunKey — одно значение автозапуска.
type RunKey struct {
	Hive string `json:"hive"`
	Path string `json:"path"`
	Name string `json:"name"`
	Data string `json:"data"`
}

// Value — значение в .reg-файле.
type Value struct {
	Name string
	Data string
}

// Collector — экспортирует Run-ветки и пишет сводный run_keys.json.
// regRoot — каталог с заранее экспортированными .reg (подмена в тестах);
// пусто — экспорт выполняется сам через reg.exe.
type Collector struct {
	regRoot string
}

// New — коллектор реестра; regRoot пусто = экспорт через reg.exe на месте.
func New(regRoot string) *Collector { return &Collector{regRoot: regRoot} }

// Name — имя источника для отчёта.
func (c *Collector) Name() string { return "registry" }

// Collect — экспорт + разбор Run-ключей; итог в <dest>/registry/run_keys.json.
func (c *Collector) Collect(ctx context.Context, dest string) []domain.Artifact {
	arts := make([]domain.Artifact, 0, len(runKeyPaths))
	sub := filepath.Join(dest, "registry")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return append(arts, domain.Artifact{Source: c.Name(), Err: err.Error()})
	}

	var keys []RunKey
	for _, rk := range append(runKeyPaths, networkListPaths...) {
		if err := ctx.Err(); err != nil {
			return arts
		}
		regPath, err := c.exportKey(ctx, sub, rk)
		if err != nil {
			arts = append(arts, domain.Artifact{Source: "registry:" + rk.hive + `\` + rk.key, Err: err.Error()})
			continue
		}
		if regPath == "" {
			continue // ветки нет на машине — тишина
		}
		for _, v := range parseRegFile(regPath) {
			keys = append(keys, RunKey{Hive: rk.hive, Path: rk.key, Name: v.Name, Data: v.Data})
		}
		arts = append(arts, domain.Artifact{
			Source: "registry:" + rk.hive + `\` + rk.key,
			Path:   filepath.ToSlash(filepath.Join("registry", filepath.Base(regPath))),
		})
	}
	if len(keys) == 0 {
		return arts
	}

	b, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return append(arts, domain.Artifact{Source: c.Name(), Err: err.Error()})
	}
	out := filepath.Join(sub, "run_keys.json")
	if err := os.WriteFile(out, b, 0o644); err != nil {
		return append(arts, domain.Artifact{Source: c.Name(), Err: err.Error()})
	}
	if fi, err := os.Stat(out); err == nil {
		arts = append(arts, domain.Artifact{Source: "registry:run_keys", Path: filepath.ToSlash(filepath.Join("registry", "run_keys.json")), Bytes: fi.Size()})
	}
	return arts
}

// exportKey — экспорт одной ветки: из regRoot (тесты) или через reg.exe.
func (c *Collector) exportKey(ctx context.Context, sub string, rk struct{ hive, key string }) (string, error) {
	hive, key := rk.hive, rk.key
	name := hive + "_" + strings.NewReplacer("\\", "_", "/", "_").Replace(key) + ".reg"
	out := filepath.Join(sub, name)

	if c.regRoot != "" {
		rel := hive + "/" + strings.ReplaceAll(key, `\`, "/") + ".reg"
		src := filepath.FromSlash(filepath.Join(c.regRoot, rel))
		if _, err := os.Stat(src); err != nil {
			// ветки нет — тишина (тестовый режим); на реальной винде reg export несуществующей ветки — норм-шум
			return "", nil
		}
		b, err := os.ReadFile(src)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(out, b, 0o644); err != nil {
			return "", err
		}
		return out, nil
	}

	cmd := exec.CommandContext(ctx, "reg", "export", hive+`\`+key, out, "/y")
	if b, err := cmd.CombinedOutput(); err != nil {
		// reg.exe пишет в OEM-кодировке (CP866) — декодируем для читаемого отчёта
		return "", fmt.Errorf("%s: %s", key, strings.TrimSpace(wincp.Decode(b)))
	}
	return out, nil
}

// parseRegFile — извлекает (имя, данные) из содержимого .reg-файла.
func parseRegContent(content string) []Value {
	var vals []Value
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, `"`) {
			continue
		}
		eq := strings.Index(line, `"=`)
		if eq < 0 {
			continue
		}
		name := strings.Trim(line[:eq+1], `"`)
		data := strings.TrimSpace(line[eq+2:])
		if i := strings.Index(data, ":"); strings.HasPrefix(data, "hex") || strings.HasPrefix(data, "dword") {
			data = data[i+1:]
		}
		data = strings.Trim(data, `"`)
		vals = append(vals, Value{Name: name, Data: data})
	}
	return vals
}

// parseRegFile — читает файл и разбирает значения.
func parseRegFile(path string) []Value {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseRegContent(string(b))
}
