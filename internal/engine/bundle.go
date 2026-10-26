package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// writeFiles confines bundle writes to the compilation directory, including
// when a reused work directory contains symbolic links.
func writeFiles(dir string, files map[string][]byte) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		clean := filepath.Clean(filepath.FromSlash(name))
		if !filepath.IsLocal(clean) || clean == "." {
			return fmt.Errorf("invalid bundle path %q", name)
		}
		if err := root.MkdirAll(filepath.Dir(clean), 0o700); err != nil {
			return err
		}
		if existing, err := root.ReadFile(clean); err == nil && bytes.Equal(existing, files[name]) {
			continue // unchanged (watch mode)
		}
		if err := root.WriteFile(clean, files[name], 0o600); err != nil {
			return err
		}
	}
	return nil
}
