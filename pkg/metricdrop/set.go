package metricdrop

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Snapshot is an immutable drop-set loaded from disk.
type Snapshot struct {
	Enabled    bool
	Generation string
	Hash       string
	Names      map[string]struct{}
	LastError  string
}

func emptySnapshot() *Snapshot {
	return &Snapshot{
		Enabled: false,
		Names:   map[string]struct{}{},
	}
}

func (s *Snapshot) Contains(name string) bool {
	if s == nil || !s.Enabled {
		return false
	}
	_, ok := s.Names[name]
	return ok
}

// Set is the process-wide drop-set. Hot path only Load()s the pointer.
type Set struct {
	cur   atomic.Pointer[Snapshot]
	dir   string
	store string
}

func NewSet(dir, storePath string) *Set {
	s := &Set{dir: dir, store: storePath}
	s.cur.Store(emptySnapshot())
	return s
}

func (s *Set) Load() *Snapshot {
	if s == nil {
		return emptySnapshot()
	}
	p := s.cur.Load()
	if p == nil {
		return emptySnapshot()
	}
	return p
}

func (s *Set) Dir() string { return s.dir }

func (s *Set) Reload() {
	if s == nil {
		return
	}
	if s.dir == "" {
		s.cur.Store(emptySnapshot())
		return
	}
	st, err := os.Stat(s.dir)
	if err != nil || !st.IsDir() {
		s.cur.Store(emptySnapshot())
		return
	}
	snap, err := loadDir(s.dir)
	if err != nil {
		prev := s.Load()
		if prev != nil && prev.Enabled && len(prev.Names) > 0 {
			cp := *prev
			cp.LastError = err.Error()
			s.cur.Store(&cp)
			return
		}
		if stored := s.readStore(); stored != nil && stored.Enabled && len(stored.Names) > 0 {
			stored.LastError = err.Error()
			s.cur.Store(stored)
			return
		}
		empty := emptySnapshot()
		empty.LastError = err.Error()
		s.cur.Store(empty)
		return
	}
	s.cur.Store(snap)
	_ = s.writeStore(snap)
}

func (s *Set) readStore() *Snapshot {
	if s == nil || s.store == "" {
		return nil
	}
	b, err := os.ReadFile(s.store)
	if err != nil {
		return nil
	}
	var payload struct {
		Enabled    bool   `json:"enabled"`
		Generation string `json:"generation"`
		Hash       string `json:"hash"`
		NamesGZ    []byte `json:"names_gz"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil
	}
	if !payload.Enabled {
		return nil
	}
	names := map[string]struct{}{}
	if len(payload.NamesGZ) > 0 {
		parsed, err := UngzipNames(payload.NamesGZ)
		if err != nil {
			return nil
		}
		for _, n := range parsed {
			if n != "" {
				names[n] = struct{}{}
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	return &Snapshot{
		Enabled:    true,
		Generation: payload.Generation,
		Hash:       payload.Hash,
		Names:      names,
	}
}

func (s *Set) writeStore(snap *Snapshot) error {
	if s.store == "" || snap == nil {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(s.store), 0755)
	names := make([]string, 0, len(snap.Names))
	for n := range snap.Names {
		names = append(names, n)
	}
	gz, err := GzipNames(names)
	if err != nil {
		return err
	}
	payload := struct {
		Enabled    bool   `json:"enabled"`
		Generation string `json:"generation"`
		Hash       string `json:"hash"`
		NamesGZ    []byte `json:"names_gz"`
	}{snap.Enabled, snap.Generation, snap.Hash, gz}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(s.store, b, 0644)
}

func loadDir(dir string) (*Snapshot, error) {
	enabledRaw, err := os.ReadFile(filepath.Join(dir, FileEnabled))
	if err != nil {
		return nil, err
	}
	enabled := strings.TrimSpace(string(enabledRaw)) == "true"
	gen, _ := os.ReadFile(filepath.Join(dir, FileGeneration))
	hash, _ := os.ReadFile(filepath.Join(dir, FileContentHash))
	gz, err := os.ReadFile(filepath.Join(dir, FileNamesGZ))
	names := map[string]struct{}{}
	if enabled {
		if err != nil {
			return nil, err
		}
		parsed, err := UngzipNames(gz)
		if err != nil {
			return nil, err
		}
		for _, n := range parsed {
			if n != "" {
				names[n] = struct{}{}
			}
		}
	}
	return &Snapshot{
		Enabled:    enabled,
		Generation: strings.TrimSpace(string(gen)),
		Hash:       strings.TrimSpace(string(hash)),
		Names:      names,
	}, nil
}

func GzipNames(names []string) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	for i, n := range names {
		if i > 0 {
			if _, err := w.Write([]byte{'\n'}); err != nil {
				return nil, err
			}
		}
		if _, err := w.Write([]byte(n)); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func UngzipNames(gz []byte) ([]string, error) {
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	return strings.Split(string(b), "\n"), nil
}

func HashUncompressed(names []string) string {
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(n)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
