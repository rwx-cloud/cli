package accesstoken

import "github.com/rwx-cloud/cli/internal/config"

const Filename = "accesstoken"

type Backend interface {
	Get() (string, error)
	Set(token string) error
}

type FileBackend struct {
	backend config.Backend
}

func NewFileBackend(backend config.Backend) *FileBackend {
	return &FileBackend{backend: backend}
}

func (f *FileBackend) Get() (string, error) {
	return f.backend.Get(Filename)
}

func (f *FileBackend) Set(token string) error {
	return f.backend.Set(Filename, token)
}

type MemoryBackend struct {
	token string
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{}
}

func (m *MemoryBackend) Get() (string, error) {
	return m.token, nil
}

func (m *MemoryBackend) Set(token string) error {
	m.token = token
	return nil
}

func Set(backend Backend, token string) error {
	return backend.Set(token)
}

func Get(backend Backend, provided string) (string, error) {
	if provided != "" {
		return provided, nil
	}

	return backend.Get()
}
