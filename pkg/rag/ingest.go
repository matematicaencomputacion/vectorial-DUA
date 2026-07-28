package rag

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// IngestOptions configures knowledge base loading.
type IngestOptions struct {
	Root     string
	Embedder Embedder
}

// IngestWalk loads all *.md under root, chunks, embeds and upserts into store.
func IngestWalk(ctx context.Context, store *Store, opt IngestOptions) (int, error) {
	if store == nil {
		return 0, fmt.Errorf("store is nil")
	}
	if opt.Embedder == nil {
		emb, err := DefaultEmbedderE()
		if err != nil {
			return 0, fmt.Errorf("embedder: %w", err)
		}
		opt.Embedder = emb
	}
	root := opt.Root
	if root == "" {
		root = "data/knowledge_base"
	}
	info, err := os.Stat(root)
	if err != nil {
		return 0, fmt.Errorf("knowledge base root: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("knowledge base root is not a directory: %s", root)
	}

	count := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		chunks := ChunkMarkdown(rel, string(raw))
		for _, c := range chunks {
			emb, err := opt.Embedder.Embed(ctx, c.Title+"\n"+c.Text)
			if err != nil {
				return fmt.Errorf("embed %s: %w", c.ID, err)
			}
			c.Embedding = emb
			store.Upsert(c)
			count++
		}
		return nil
	})
	return count, err
}

// MustIngest is a helper for demos; panics on error.
func MustIngest(store *Store, root string) int {
	emb, err := DefaultEmbedderE()
	if err != nil {
		panic(err)
	}
	n, err := IngestWalk(context.Background(), store, IngestOptions{Root: root, Embedder: emb})
	if err != nil {
		panic(err)
	}
	return n
}
