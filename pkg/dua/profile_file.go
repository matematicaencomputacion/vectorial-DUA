package dua

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	profileSnapshotVersion = 1
	defaultFlushDebounce   = time.Second
)

// profileSnapshotFile is the on-disk JSON schema for FileProfileStore.
type profileSnapshotFile struct {
	Version  int                  `json:"version"`
	VeDims   int                  `json:"ve_dims"`
	Profiles map[string][]float32 `json:"profiles"`
}

// FileProfileStore decorates an in-memory ProfileStore with debounced JSON snapshots.
// It implements ProfileRepository. Snapshot() on the inner store is used only for flush.
type FileProfileStore struct {
	mem      *ProfileStore
	path     string
	debounce time.Duration

	mu             sync.Mutex
	dirty          bool
	flushScheduled bool
	closed         bool

	writes atomic.Int64 // test observability: successful durable flushes
}

// NewFileProfileStore opens (or creates) a JSON snapshot at path.
// Corrupt or ve_dims-mismatched files are discarded with a log; startup never fails
// solely because of a bad snapshot.
func NewFileProfileStore(path string) (*FileProfileStore, error) {
	return NewFileProfileStoreWithDebounce(path, defaultFlushDebounce)
}

// NewFileProfileStoreWithDebounce is like NewFileProfileStore with a custom debounce.
func NewFileProfileStoreWithDebounce(path string, debounce time.Duration) (*FileProfileStore, error) {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return nil, fmt.Errorf("FileProfileStore: path is required")
	}
	if debounce <= 0 {
		debounce = defaultFlushDebounce
	}
	f := &FileProfileStore{
		mem:      NewProfileStore(),
		path:     path,
		debounce: debounce,
	}
	if err := f.load(); err != nil {
		log.Printf("profile store: snapshot load skipped (%s): %v — starting empty", path, err)
	}
	return f, nil
}

func (f *FileProfileStore) Get(studentID string) []float32 {
	return f.mem.Get(studentID)
}

func (f *FileProfileStore) Apply(studentID string, delta []float32) ([]float32, error) {
	next, err := f.mem.Apply(studentID, delta)
	if err != nil {
		return nil, err
	}
	f.scheduleFlush()
	return next, nil
}

// WriteCount returns how many successful durable flushes have completed (tests).
func (f *FileProfileStore) WriteCount() int64 {
	return f.writes.Load()
}

// Flush writes the current snapshot to disk immediately if dirty (or always if force).
func (f *FileProfileStore) Flush() error {
	return f.flush(false)
}

// Close flushes any pending changes and prevents further writes.
func (f *FileProfileStore) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return f.flush(true)
}

func (f *FileProfileStore) scheduleFlush() {
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
		if err := f.flush(false); err != nil {
			log.Printf("profile store: flush failed (%s): %v", f.path, err)
		}
	})
}

func (f *FileProfileStore) flush(force bool) error {
	f.mu.Lock()
	if !force && !f.dirty {
		f.mu.Unlock()
		return nil
	}
	f.dirty = false
	path := f.path
	f.mu.Unlock()

	snap := f.mem.Snapshot()
	doc := profileSnapshotFile{
		Version:  profileSnapshotVersion,
		VeDims:   VeDims,
		Profiles: snap,
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir snapshot dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp snapshot: %w", err)
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
		return fmt.Errorf("write temp snapshot: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp snapshot: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename snapshot: %w", err)
	}
	cleanup = false
	f.writes.Add(1)
	return nil
}

func (f *FileProfileStore) load() error {
	raw, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		return fmt.Errorf("empty snapshot file")
	}

	var doc profileSnapshotFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("corrupt JSON: %w", err)
	}
	if doc.Version != profileSnapshotVersion {
		return fmt.Errorf("unsupported snapshot version %d (want %d)", doc.Version, profileSnapshotVersion)
	}
	if doc.VeDims != VeDims {
		return fmt.Errorf("ve_dims mismatch: snapshot=%d runtime=%d (discarding file)", doc.VeDims, VeDims)
	}

	loaded := make(map[string][]float32)
	for id, ve := range doc.Profiles {
		if len(ve) != VeDims {
			log.Printf("profile store: discarding student %q: dims=%d want=%d", id, len(ve), VeDims)
			continue
		}
		loaded[id] = ve
	}
	f.mem.ReplaceAll(loaded)
	return nil
}

var _ ProfileRepository = (*FileProfileStore)(nil)
