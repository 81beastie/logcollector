package domain

import (
	"strings"
	"testing"
	"time"
)

func TestArtifactOk_NoErrorIsOk(t *testing.T) {
	if !(Artifact{Source: "x"}.Ok() && !Artifact{Err: "нет доступа"}.Ok()) {
		t.Error("Ok() должен зависеть только от пустой ошибки")
	}
}

func TestCounts_OkFailedBytes(t *testing.T) {
	b := Bundle{Artifacts: []Artifact{
		{Source: "a", Bytes: 100},
		{Source: "b", Bytes: 2048},
		{Source: "c", Err: "отказано"},
	}}
	ok, failed, total := b.Counts()
	if ok != 2 || failed != 1 || total != 2148 {
		t.Errorf("Counts = %d/%d/%d, ожидала 2/1/2148", ok, failed, total)
	}
}

func TestCounts_EmptyBundle(t *testing.T) {
	if ok, failed, total := (Bundle{}).Counts(); ok != 0 || failed != 0 || total != 0 {
		t.Error("пустой бандл: ожидала нули")
	}
}

func TestReport_ContainsSummaryAndFailures(t *testing.T) {
	b := Bundle{
		Host:      "HOST1",
		StartedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		OutPath:   `C:\triage\HOST1.zip`,
		Artifacts: []Artifact{
			{Source: "eventlog:Security", Path: "eventlog/Security.evtx", Bytes: 5 << 20},
			{Source: "anydesk", Err: "каталог не найден"},
		},
	}
	r := b.Report()
	for _, want := range []string{
		"HOST1",
		`C:\triage\HOST1.zip`,
		"собрано: 1, ошибок: 1",
		"5.0M",
		"eventlog:Security",
		"!",
		"каталог не найден",
	} {
		if !strings.Contains(r, want) {
			t.Errorf("отчёт не содержит %q:\n%s", want, r)
		}
	}
}

func TestSortedArtifacts_StableOrder(t *testing.T) {
	b := Bundle{Artifacts: []Artifact{{Source: "zeta"}, {Source: "alpha"}, {Source: "mid"}}}
	got := b.SortedArtifacts()
	if got[0].Source != "alpha" || got[1].Source != "mid" || got[2].Source != "zeta" {
		t.Errorf("порядок: %v, ожидала alpha/mid/zeta", got)
	}
	if len(b.Artifacts) != 3 || b.Artifacts[0].Source != "zeta" {
		t.Error("SortedArtifacts не должна менять исходный слайс")
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		512:     "512B",
		2048:    "2.0K",
		5 << 20: "5.0M",
		3 << 30: "3.0G",
	}
	for n, want := range cases {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %q, ожидала %q", n, got, want)
		}
	}
}

func TestEventLogNames_ContainsForensicCore(t *testing.T) {
	for _, want := range []string{"Security", "System", "Microsoft-Windows-TerminalServices-RemoteConnectionManager/Operational", "Microsoft-Windows-PowerShell/Operational", "Microsoft-Windows-WLAN-AutoConfig/Operational"} {
		found := false
		for _, n := range EventLogNames {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("EventLogNames не содержит %q", want)
		}
	}
}

func TestRemoteSources_UniqueNamesAndGlobs(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range RemoteSources {
		if s.Name == "" {
			t.Error("источник без имени")
		}
		if seen[s.Name] {
			t.Errorf("дубль имени %q", s.Name)
		}
		seen[s.Name] = true
		if len(s.Globs) == 0 {
			t.Errorf("источник %q без glob-паттернов", s.Name)
		}
		for _, g := range s.Globs {
			if !strings.ContainsRune(g, '\\') {
				t.Errorf("glob %q источника %q без разделителя пути", g, s.Name)
			}
		}
	}
	for _, want := range []string{"anydesk", "ammyy", "rudesktop", "radmin", "teamviewer", "meshagent",
		"aeroadmin", "ultraviewer", "dwservice", "logmein"} {
		if !seen[want] {
			t.Errorf("RemoteSources не содержит %q", want)
		}
	}
}

func TestAVSources_UniqueNamesAndGlobs(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range AVSources {
		if seen[s.Name] {
			t.Errorf("дубль имени %q", s.Name)
		}
		seen[s.Name] = true
		if len(s.Globs) == 0 {
			t.Errorf("источник %q без glob-паттернов", s.Name)
		}
	}
	for _, want := range []string{"kaspersky", "drweb"} {
		if !seen[want] {
			t.Errorf("AVSources не содержит %q", want)
		}
	}
}

func TestAllFileSources_CoversRemoteAndAV(t *testing.T) {
	all := AllFileSources()
	if len(all) != len(RemoteSources)+len(AVSources) {
		t.Errorf("AllFileSources = %d, ожидала %d (Remote+AV)", len(all), len(RemoteSources)+len(AVSources))
	}
	seen := map[string]bool{}
	for _, s := range all {
		if seen[s.Name] {
			t.Errorf("дубль %q между реестрами", s.Name)
		}
		seen[s.Name] = true
	}
}

func TestRemoteSources_RuDesktopCoversDocPaths(t *testing.T) {
	var rd *RemoteSource
	for i := range RemoteSources {
		if RemoteSources[i].Name == "rudesktop" {
			rd = &RemoteSources[i]
		}
	}
	if rd == nil {
		t.Fatal("rudesktop нет в реестре")
	}
	joined := strings.Join(rd.Globs, "\n")
	for _, want := range []string{
		`%SystemRoot%\ServiceProfiles\LocalService\AppData\Roaming\RuDesktop\logs\service\*.log`,
		`%AppData%\RuDesktop\logs\*\*.log`,
		`%AppData%\RuDesktop\..\RuDesktop.toml`,
		`%AppData%\RuDesktop\..\peers\*.toml`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("глоб rudesktop не покрывает путь из документации: %q", want)
		}
	}
}

func TestRemoteSources_MeshAgentCoversDocPaths(t *testing.T) {
	var ma *RemoteSource
	for i := range RemoteSources {
		if RemoteSources[i].Name == "meshagent" {
			ma = &RemoteSources[i]
		}
	}
	if ma == nil {
		t.Fatal("meshagent нет в реестре")
	}
	joined := strings.Join(ma.Globs, "\n")
	for _, want := range []string{
		`%ProgramFiles%\Mesh Agent\*.log`,
		`%ProgramFiles%\Mesh Agent\meshagent.db`,
		`%ProgramFiles%\Mesh Agent\meshagent.msh`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("глоб meshagent не покрывает путь из документации: %q", want)
		}
	}
}
