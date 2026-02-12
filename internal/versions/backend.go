package versions

import "github.com/rwx-cloud/cli/internal/config"

type Backend interface {
	Get() (string, error)
	Set(version string) error
}

type FileBackend struct {
	backend config.Backend
}

func NewFileBackend(backend config.Backend) *FileBackend {
	return &FileBackend{backend: backend}
}

func (f *FileBackend) Get() (string, error) {
	return f.backend.Get(latestVersionFilename)
}

func (f *FileBackend) Set(version string) error {
	return f.backend.Set(latestVersionFilename, version)
}

type MemoryBackend struct {
	version string
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{}
}

func (m *MemoryBackend) Get() (string, error) {
	return m.version, nil
}

func (m *MemoryBackend) Set(version string) error {
	m.version = version
	return nil
}
