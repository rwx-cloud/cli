package docstoken

import (
	"fmt"
	"strings"

	"github.com/rwx-cloud/cli/internal/config"
)

const Filename = "docs_token"

type DocsToken struct {
	Token     string
	AuthToken string
}

type Backend interface {
	Get() (DocsToken, error)
	Set(dt DocsToken) error
}

type FileBackend struct {
	backend config.Backend
}

func NewFileBackend(backend config.Backend) *FileBackend {
	return &FileBackend{backend: backend}
}

func (f *FileBackend) Get() (DocsToken, error) {
	raw, err := f.backend.Get(Filename)
	if err != nil {
		return DocsToken{}, err
	}

	if raw == "" {
		return DocsToken{}, nil
	}

	parts := strings.SplitN(raw, "\n", 2)
	dt := DocsToken{Token: parts[0]}
	if len(parts) > 1 {
		dt.AuthToken = parts[1]
	}

	return dt, nil
}

func (f *FileBackend) Set(dt DocsToken) error {
	return f.backend.Set(Filename, fmt.Sprintf("%s\n%s", dt.Token, dt.AuthToken))
}

type MemoryBackend struct {
	docsToken DocsToken
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{}
}

func (m *MemoryBackend) Get() (DocsToken, error) {
	return m.docsToken, nil
}

func (m *MemoryBackend) Set(dt DocsToken) error {
	m.docsToken = dt
	return nil
}
