// Package remotefiles — раскрытие %Переменных% в путях и поиск файлов по glob-паттернам.
package remotefiles

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Expand — раскрывает %Var% в строке пути через переменные окружения;
// неизвестные переменные остаются как есть (путь потом просто не найдётся).
func Expand(s string) string {
	var sb strings.Builder
	for {
		i := strings.IndexByte(s, '%')
		if i < 0 {
			sb.WriteString(s)
			return sb.String()
		}
		j := strings.IndexByte(s[i+1:], '%')
		if j < 0 {
			sb.WriteString(s)
			return sb.String()
		}
		key := s[i+1 : i+1+j]
		if v, ok := os.LookupEnv(key); ok {
			sb.WriteString(s[:i])
			sb.WriteString(v)
		} else {
			sb.WriteString(s[:i+j+2])
		}
		s = s[i+j+2:]
	}
}

// Resolve — превращает список glob-паттернов в отсортированный список существующих файлов.
// Несуществующие паттерны — не ошибка: инструмента может не быть на машине.
func Resolve(globs []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, g := range globs {
		matches, err := filepath.Glob(toNative(Expand(g)))
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if fi, err := os.Stat(m); err == nil && !fi.IsDir() && !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// toNative — виндовые паттерны с "\" работают на любой платформе:
// заменяем разделитель на нативный (на Windows filepath.Sep == '\', замена ничего не меняет).
func toNative(p string) string {
	return strings.ReplaceAll(p, `\`, string(filepath.Separator))
}
