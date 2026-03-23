package versions

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rwx-cloud/rwx/internal/config"
)

type Backend interface {
	Get() (string, error)
	Set(version string) error
	ModTime() (time.Time, error)
}

type FileBackend struct {
	backend  config.Backend
	filename string
}

func NewFileBackend(backend config.Backend) *FileBackend {
	return &FileBackend{backend: backend, filename: latestVersionFilename}
}

func NewSkillFileBackend(backend config.Backend) *FileBackend {
	return &FileBackend{backend: backend, filename: latestSkillVersionFilename}
}

func (f *FileBackend) Get() (string, error) {
	return f.backend.Get(f.filename)
}

func (f *FileBackend) Set(version string) error {
	return f.backend.Set(f.filename, version)
}

func (f *FileBackend) ModTime() (time.Time, error) {
	fb, ok := f.backend.(*config.FileBackend)
	if !ok {
		return time.Time{}, os.ErrNotExist
	}

	info, err := os.Stat(filepath.Join(fb.PrimaryDirectory, f.filename))
	if err != nil {
		return time.Time{}, err
	}

	return info.ModTime(), nil
}

type MemoryBackend struct {
	version string
	modTime time.Time
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{}
}

func (m *MemoryBackend) Get() (string, error) {
	return m.version, nil
}

func (m *MemoryBackend) Set(version string) error {
	m.version = version
	m.modTime = time.Now()
	return nil
}

func (m *MemoryBackend) ModTime() (time.Time, error) {
	if m.version == "" {
		return time.Time{}, os.ErrNotExist
	}
	return m.modTime, nil
}

// SetModTime overrides the mod time for testing cache TTL behavior.
func (m *MemoryBackend) SetModTime(t time.Time) {
	m.modTime = t
}
