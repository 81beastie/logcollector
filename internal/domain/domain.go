// Package domain — модели триажного сборщика логов: артефакты, источники, результаты.
package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Artifact — один собранный артефакт (файл или экспорт журнала).
type Artifact struct {
	Source string // имя источника: "eventlog:Security", "anydesk"
	Path   string // путь внутри бандла: "eventlog/Security.evtx"
	Bytes  int64  // размер
	Err    string // пусто = успех; ошибка как данные
}

// Ok — артефакт собран без ошибки.
func (a Artifact) Ok() bool { return a.Err == "" }

// CollectError — зафиксированный отказ источника: попадает в манифест бандла,
// чтобы «нет журнала в архиве» было отличимо от «журналов не было».
type CollectError struct {
	Source string `json:"source"`
	Err    string `json:"error"`
}

// Errors — все отказы бандла как данные.
func (b Bundle) Errors() []CollectError {
	var out []CollectError
	for _, a := range b.Artifacts {
		if !a.Ok() {
			out = append(out, CollectError{Source: a.Source, Err: a.Err})
		}
	}
	return out
}

// Bundle — итог сбора на одном хосте.
type Bundle struct {
	Host      string    // имя компьютера
	StartedAt time.Time // момент старта сбора
	OutPath   string    // путь к итоговому zip
	Artifacts []Artifact
}

// Counts — сводка по бандлу: собрано/ошибки/байты.
func (b Bundle) Counts() (ok, failed, totalBytes int64) {
	for _, a := range b.Artifacts {
		if a.Ok() {
			ok++
			totalBytes += a.Bytes
		} else {
			failed++
		}
	}
	return ok, failed, totalBytes
}

// Report — человекочитаемый отчёт по итогам сбора.
func (b Bundle) Report() string {
	ok, failed, total := b.Counts()
	var sb strings.Builder
	fmt.Fprintf(&sb, "хост: %s\n", b.Host)
	fmt.Fprintf(&sb, "итог: %s\n", b.OutPath)
	fmt.Fprintf(&sb, "собрано: %d, ошибок: %d, объём: %s\n\n", ok, failed, HumanBytes(total))
	for _, a := range b.Artifacts {
		mark := "+"
		if !a.Ok() {
			mark = "!"
		}
		fmt.Fprintf(&sb, "%s %-28s %10s  %s\n", mark, a.Source, HumanBytes(a.Bytes), a.Err)
	}
	return sb.String()
}

// SortedArtifacts — артефакты, отсортированные по источнику (стабильный отчёт).
func (b Bundle) SortedArtifacts() []Artifact {
	out := append([]Artifact(nil), b.Artifacts...)
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// HumanBytes — размер в человекочитаемом виде.
func HumanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// EventLogNames — журналы Windows для экспорта (наш набор из кейса БФК).
var EventLogNames = []string{
	"Security",
	"System",
	"Application",
	"Microsoft-Windows-TerminalServices-RemoteConnectionManager/Operational",
	"Microsoft-Windows-TerminalServices-LocalSessionManager/Operational",
	"Microsoft-Windows-PowerShell/Operational",
	"Microsoft-Windows-WinRM/Operational",
	"Microsoft-Windows-TaskScheduler/Operational",
	"Microsoft-Windows-WMI-Activity/Operational",
	"Microsoft-Windows-Sysmon/Operational",
	"Microsoft-Windows-WLAN-AutoConfig/Operational", // Wi-Fi: подключения с SSID (8001/8003)
}

// RemoteSource — след удалённого доступа: каталоги для копирования (glob-паттерны).
type RemoteSource struct {
	Name  string   // "anydesk"
	Globs []string // "%ProgramData%\\AnyDesk\\*trace*.txt"
	Kind  string   // категория для отчёта: remote, av, mesh
}

// RemoteSources — реестр следов удалёнки (переменные окружения раскроет инфраструктура).
var RemoteSources = []RemoteSource{
	{Name: "anydesk", Kind: "remote", Globs: []string{
		`%ProgramData%\AnyDesk\connection_trace.txt`,
		`%ProgramData%\AnyDesk\ad.trace`,
		`%ProgramData%\AnyDesk\*.trace`,
		`%AppData%\AnyDesk\*.trace`,
	}},
	{Name: "ammyy", Kind: "remote", Globs: []string{
		`%SystemRoot%\ammyy\*.log`,
		`%ProgramFiles%\Ammyy Admin\*.log`,
	}},
	{Name: "rudesktop", Kind: "remote", Globs: []string{
		// сервисные логи (официальная документация RuDesktop):
		// %windir%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\<команда>\rudesktop_rCURRENT.log
		`%SystemRoot%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\service\*.log`,
		`%SystemRoot%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\server\*.log`,
		`%SystemRoot%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\rendezvous\*.log`,
		`%SystemRoot%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\get-id\*.log`,
		`%SystemRoot%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\register-log\*.log`,
		`%SystemRoot%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\unregister-log\*.log`,
		// пользовательские логи
		`%AppData%\RuDesktop\logs\*.log`,
		`%AppData%\RuDesktop\logs\*\*.log`,
		// конфиги: ID устройств и peers — с кем соединялись
		`%AppData%\RuDesktop\..\RuDesktop.toml`,
		`%AppData%\RuDesktop\..\RuDesktop2.toml`,
		`%AppData%\RuDesktop\..\peers\*.toml`,
		`%AppData%\RuDesktop\config\*`,
	}},
	{Name: "radmin", Kind: "remote", Globs: []string{
		`%SystemRoot%\SysWOW64\rserver30\*.log`,
		`%ProgramFiles%\Radmin VPN\*.log`,
	}},
	{Name: "teamviewer", Kind: "remote", Globs: []string{
		`%ProgramData%\TeamViewer\*.log`,
		`%AppData%\TeamViewer\*.log`,
		`%ProgramFiles%\TeamViewer\*.log`,
	}},
	{Name: "meshagent", Kind: "mesh", Globs: []string{
		// официальная документация MeshCentral: всё лежит в каталоге установки агента
		`%ProgramFiles%\Mesh Agent\*.log`,
		`%ProgramFiles%\Mesh Agent\meshagent.db`,  // база агента: история команд и соединений
		`%ProgramFiles%\Mesh Agent\meshagent.msh`, // конфиг сервера, с которым агент соединялся
		`%ProgramData%\Mesh Agent\*.log`,
	}},
	{Name: "supremo", Kind: "remote", Globs: []string{
		`%ProgramData%\Supremo\*.log`,
	}},
	{Name: "aeroadmin", Kind: "remote", Globs: []string{
		`%ProgramData%\AeroAdmin\*.log`,
		`%AppData%\AeroAdmin\*.log`,
	}},
	{Name: "ultraviewer", Kind: "remote", Globs: []string{
		`%ProgramData%\UltraViewer\*.log`,
		`%ProgramFiles(x86)%\UltraViewer\*.log`,
		`%AppData%\UltraViewer\*.log`,
	}},
	{Name: "dwservice", Kind: "remote", Globs: []string{
		`%ProgramData%\DWAgent\*.log`,
		`%ProgramData%\DWAgent\*.conf`,
		`%SystemRoot%\Temp\dwagent*.log`,
	}},
	{Name: "logmein", Kind: "remote", Globs: []string{
		`%ProgramData%\LogMeIn\*.log`,
		`%ProgramData%\LogMeIn Rescue\*.log`,
		`%ProgramFiles(x86)%\LogMeIn\*.log`,
	}},
}

// AVSources — следы антивирусов: логи детектов и отключения защиты.
var AVSources = []RemoteSource{
	{Name: "kaspersky", Kind: "av", Globs: []string{
		`%ProgramData%\Kaspersky Lab\*.log`,
		`%ProgramData%\Kaspersky Lab\Report\*`,
		`%ProgramData%\Kaspersky Lab\KES\*\Report\*`,
		`%ALLUSERSPROFILE%\Kaspersky Lab\Kaspersky*\*.log`,
	}},
	{Name: "drweb", Kind: "av", Globs: []string{
		`%ProgramData%\DrWeb\*.log`,
		`%ALLUSERSPROFILE%\Doctor Web\*.log`,
		`%ProgramFiles%\DrWeb\*.log`,
	}},
	{Name: "avg", Kind: "av", Globs: []string{
		`%ProgramData%\AVG\*.log`,
		`%ProgramData%\AVG\Antivirus\report\*`,
	}},
	{Name: "avast", Kind: "av", Globs: []string{
		`%ProgramData%\AVAST Software\Avast\*.log`,
		`%ProgramData%\Avast Software\Avast\log\*`,
	}},
	{Name: "eset", Kind: "av", Globs: []string{
		`%ProgramData%\ESET\*.log`,
		`%ALLUSERSPROFILE%\ESET\ByteSquads\Logs\*`,
	}},
	{Name: "windowsdefender", Kind: "av", Globs: []string{
		`%ProgramData%\Microsoft\Windows Defender\Support\MPLog-*`,
		`%ProgramData%\Microsoft\Windows Defender\Scans\History\*`,
	}},
}

// AllFileSources — единый реестр файловых источников (удалёнка + антивирусы).
func AllFileSources() []RemoteSource {
	all := make([]RemoteSource, 0, len(RemoteSources)+len(AVSources))
	all = append(all, RemoteSources...)
	all = append(all, AVSources...)
	return all
}
