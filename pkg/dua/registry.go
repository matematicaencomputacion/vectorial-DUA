package dua

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Registry is a concurrent in-memory store of interactive video nodes.
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]*InteractiveVideoNode
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{nodes: make(map[string]*InteractiveVideoNode)}
}

// Put validates and stores a node (replace by node_id).
func (r *Registry) Put(n *InteractiveVideoNode) error {
	if err := n.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[n.NodeID] = n.Clone()
	return nil
}

// Get returns a clone of the node or nil.
func (r *Registry) Get(nodeID string) (*InteractiveVideoNode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.nodes[nodeID]
	if !ok {
		return nil, false
	}
	return n.Clone(), true
}

// Len returns the number of registered interactive nodes.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.nodes)
}

// AppendButton adds a live button to an existing node.
func (r *Registry) AppendButton(nodeID string, btn InteractiveButton) (*InteractiveVideoNode, error) {
	if err := btn.Validate(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("interactive node not found: %s", nodeID)
	}
	for _, existing := range n.Botonera {
		if existing.IDBtn == btn.IDBtn {
			return nil, fmt.Errorf("button id already exists: %s", btn.IDBtn)
		}
	}
	n.Botonera = append(n.Botonera, btn)
	if err := n.Validate(); err != nil {
		// rollback last append
		n.Botonera = n.Botonera[:len(n.Botonera)-1]
		return nil, err
	}
	return n.Clone(), nil
}

// LoadDir loads all *.json interactive node seeds from a directory.
func (r *Registry) LoadDir(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return count, err
		}
		var node InteractiveVideoNode
		if err := json.Unmarshal(raw, &node); err != nil {
			return count, fmt.Errorf("%s: %w", path, err)
		}
		if err := r.Put(&node); err != nil {
			return count, fmt.Errorf("%s: %w", path, err)
		}
		count++
	}
	return count, nil
}

// ForEach invokes fn with a clone of each registered node.
func (r *Registry) ForEach(fn func(*InteractiveVideoNode)) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, n := range r.nodes {
		fn(n.Clone())
	}
}

// EnabledFromEnv reads AVLP_INTERACTIVE_NODES (default true).
func EnabledFromEnv() bool {
	v := strings.TrimSpace(os.Getenv("AVLP_INTERACTIVE_NODES"))
	if v == "" {
		return true
	}
	return !strings.EqualFold(v, "false") && v != "0"
}
