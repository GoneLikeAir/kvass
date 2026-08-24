package metricdrop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSet_ContainsUsesMap(t *testing.T) {
	s := NewSet("", "")
	snap := &Snapshot{Enabled: true, Names: map[string]struct{}{"idle_metric": {}}}
	s.cur.Store(snap)
	if _, ok := snap.Names["idle_metric"]; !ok {
		t.Fatal("expected map membership")
	}
	if !s.Load().Contains("idle_metric") {
		t.Fatal("Contains should be O(1) map lookup")
	}
	if s.Load().Contains("keep_metric") {
		t.Fatal("unknown name must pass")
	}
}

func TestSet_NoDirIgnoresStoreFile(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "metric-drop-set.json")
	prev := NewSet(filepath.Join(dir, "present"), store)
	_ = os.MkdirAll(prev.dir, 0755)
	_ = os.WriteFile(filepath.Join(prev.dir, FileEnabled), []byte("true\n"), 0644)
	_ = os.WriteFile(filepath.Join(prev.dir, FileGeneration), []byte("1\n"), 0644)
	gz, _ := GzipNames([]string{"idle_metric"})
	_ = os.WriteFile(filepath.Join(prev.dir, FileNamesGZ), gz, 0644)
	prev.Reload()
	if !prev.Load().Contains("idle_metric") {
		t.Fatal("setup")
	}

	missing := NewSet(filepath.Join(dir, "missing"), store)
	missing.Reload()
	if missing.Load().Enabled {
		t.Fatal("missing dir must not read store")
	}
	if missing.Load().Contains("idle_metric") {
		t.Fatal("must not filter when dir missing")
	}
}

func TestSet_DisableOverridesStore(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store.json")
	dropDir := filepath.Join(dir, "drop")
	_ = os.MkdirAll(dropDir, 0755)
	s := NewSet(dropDir, store)
	gz, _ := GzipNames([]string{"idle_metric"})
	_ = os.WriteFile(filepath.Join(dropDir, FileEnabled), []byte("true\n"), 0644)
	_ = os.WriteFile(filepath.Join(dropDir, FileGeneration), []byte("5\n"), 0644)
	_ = os.WriteFile(filepath.Join(dropDir, FileContentHash), []byte("aaa\n"), 0644)
	_ = os.WriteFile(filepath.Join(dropDir, FileNamesGZ), gz, 0644)
	s.Reload()
	if !s.Load().Contains("idle_metric") {
		t.Fatal("enabled")
	}
	_ = os.WriteFile(filepath.Join(dropDir, FileEnabled), []byte("false\n"), 0644)
	_ = os.WriteFile(filepath.Join(dropDir, FileContentHash), []byte("bbb\n"), 0644)
	s.Reload()
	if s.Load().Enabled || s.Load().Contains("idle_metric") {
		t.Fatal("disable must clear set")
	}
}

func TestSet_IncompleteDirKeepsPrevious(t *testing.T) {
	dir := t.TempDir()
	dropDir := filepath.Join(dir, "drop")
	_ = os.MkdirAll(dropDir, 0755)
	s := NewSet(dropDir, filepath.Join(dir, "store.json"))
	gz, _ := GzipNames([]string{"idle_metric"})
	_ = os.WriteFile(filepath.Join(dropDir, FileEnabled), []byte("true\n"), 0644)
	_ = os.WriteFile(filepath.Join(dropDir, FileNamesGZ), gz, 0644)
	s.Reload()
	_ = os.WriteFile(filepath.Join(dropDir, FileEnabled), []byte("true\n"), 0644)
	_ = os.Remove(filepath.Join(dropDir, FileNamesGZ))
	_ = os.WriteFile(filepath.Join(dropDir, FileNamesGZ), []byte("not-gzip"), 0644)
	s.Reload()
	if !s.Load().Contains("idle_metric") {
		t.Fatal("corrupt names.gz should keep previous enabled set")
	}
	if s.Load().LastError == "" {
		t.Fatal("expected last error")
	}
}
