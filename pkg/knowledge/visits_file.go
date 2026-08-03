package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	visitSnapshotVersion = 1
	defaultVisitDebounce = time.Second
)

// visitSnapshotFile is the on-disk JSON schema for FileConceptVisitStore.
type visitSnapshotFile struct {
	Version int                          `json:"version"`
	Visits  map[string]map[string]string `json:"visits"` // student → conceptID → RFC3339
}

// FileConceptVisitStore decorates MemoryConceptVisitStore with debounced JSON snapshots.
type FileConceptVisitStore struct {
	mem      *MemoryConceptVisitStore
	path     string
	debounce time.Duration
	Logf     Logf

	mu             sync.Mutex
	dirty          bool
	flushScheduled bool
	closed         bool
	writes         atomic.Int64
}

// NewFileConceptVisitStore opens (or creates) a JSON snapshot at path.
func NewFileConceptVisitStore(path string) (*FileConceptVisitStore, error) {
	return NewFileConceptVisitStoreWithDebounce(path, defaultVisitDebounce)
}

// NewFileConceptVisitStoreWithDebounce is like NewFileConceptVisitStore with custom debounce.
func NewFileConceptVisitStoreWithDebounce(path string, debounce time.Duration) (*FileConceptVisitStore, error) {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return nil, fmt.Errorf("FileConceptVisitStore: path is required")
	}
	if debounce <= 0 {
		debounce = defaultVisitDebounce
	}
	f := &FileConceptVisitStore{
		mem:      NewMemoryConceptVisitStore(),
		path:     path,
		debounce: debounce,
	}
	if err := f.load(); err != nil {
		if f.Logf != nil {
			f.Logf("concept visit store: snapshot load skipped (%s): %v — starting empty", path, err)
		}
	}
	return f, nil
}

// RecordVisit implements ConceptVisitStore and schedules a durable flush.
func (f *FileConceptVisitStore) RecordVisit(ctx context.Context, studentID string, id ConceptID) error {
	if err := f.mem.RecordVisit(ctx, studentID, id); err != nil {
		return err
	}
	f.scheduleFlush()
	return nil
}

// Visited implements ConceptVisitStore.
func (f *FileConceptVisitStore) Visited(ctx context.Context, studentID string) (map[ConceptID]time.Time, error) {
	return f.mem.Visited(ctx, studentID)
}

// HasVisited implements ConceptVisitStore.
func (f *FileConceptVisitStore) HasVisited(ctx context.Context, studentID string, id ConceptID) (bool, error) {
	return f.mem.HasVisited(ctx, studentID, id)
}

// WriteCount returns successful durable flushes (tests).
func (f *FileConceptVisitStore) WriteCount() int64 { return f.writes.Load() }

// Flush writes the snapshot immediately if dirty (or always if force via Close).
func (f *FileConceptVisitStore) Flush() error { return f.flush(false) }

// Close flushes pending changes and prevents further writes.
func (f *FileConceptVisitStore) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return f.flush(true)
}

func (f *FileConceptVisitStore) scheduleFlush() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.dirty = true
	if f.flushScheduled {
		return
	}
	f.flushScheduled = true
	delay := f.debounce
	time.AfterFunc(delay, func() {
		f.mu.Lock()
		f.flushScheduled = false
		f.mu.Unlock()
		if err := f.flush(false); err != nil && f.Logf != nil {
			f.Logf("concept visit store: flush failed (%s): %v", f.path, err)
		}
	})
}

func (f *FileConceptVisitStore) flush(force bool) error {
	f.mu.Lock()
	if !force && !f.dirty {
		f.mu.Unlock()
		return nil
	}
	f.dirty = false
	path := f.path
	f.mu.Unlock()

	if err := f.writeSnapshot(path); err != nil {
		f.mu.Lock()
		f.dirty = true
		closed := f.closed
		f.mu.Unlock()
		if !closed {
			f.scheduleFlush()
		}
		return err
	}
	f.writes.Add(1)
	return nil
}

func (f *FileConceptVisitStore) writeSnapshot(path string) error {
	f.mem.mu.RLock()
	doc := visitSnapshotFile{
		Version: visitSnapshotVersion,
		Visits:  make(map[string]map[string]string, len(f.mem.visits)),
	}
	for student, concepts := range f.mem.visits {
		inner := make(map[string]string, len(concepts))
		for id, at := range concepts {
			inner[string(id)] = at.UTC().Format(time.RFC3339Nano)
		}
		doc.Visits[student] = inner
	}
	f.mem.mu.RUnlock()

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal visit snapshot: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir visit snapshot dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp visit snapshot: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp visit snapshot: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp visit snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp visit snapshot: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename visit snapshot: %w", err)
	}
	cleanup = false
	return nil
}

func (f *FileConceptVisitStore) load() error {
	raw, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		return fmt.Errorf("empty visit snapshot")
	}
	var doc visitSnapshotFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("corrupt visit JSON: %w", err)
	}
	if doc.Version != visitSnapshotVersion {
		return fmt.Errorf("unsupported visit snapshot version %d (want %d)", doc.Version, visitSnapshotVersion)
	}
	f.mem.mu.Lock()
	defer f.mem.mu.Unlock()
	f.mem.visits = make(map[string]map[ConceptID]time.Time)
	for student, concepts := range doc.Visits {
		inner := make(map[ConceptID]time.Time, len(concepts))
		for rawID, stamp := range concepts {
			id, err := NormalizeConceptRef(rawID)
			if err != nil {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, stamp)
			if err != nil {
				at, err = time.Parse(time.RFC3339, stamp)
				if err != nil {
					continue
				}
			}
			inner[id] = at
		}
		if len(inner) > 0 {
			f.mem.visits[student] = inner
		}
	}
	return nil
}

var _ ConceptVisitStore = (*FileConceptVisitStore)(nil)
var _ ConceptVisitStore = (*MemoryConceptVisitStore)(nil)
