package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileBackend stores each key as a JSON file under dir at <dir>/<key>.json.
// Writes are atomic via tmp+rename.
type FileBackend struct {
	dir string
}

func NewFileBackend(dir string) (*FileBackend, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	return &FileBackend{dir: dir}, nil
}

func (f *FileBackend) path(key string) string {
	return filepath.Join(f.dir, key+".json")
}

func (f *FileBackend) Get(key string) ([]byte, error) {
	data, err := os.ReadFile(f.path(key))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", f.path(key), err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	return data, nil
}

func (f *FileBackend) Put(key string, value []byte) error {
	path := f.path(key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, value, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func (f *FileBackend) Close() error { return nil }
