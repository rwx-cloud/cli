package config

type MemoryBackend struct {
	data map[string]string
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{data: make(map[string]string)}
}

func (m *MemoryBackend) Get(filename string) (string, error) {
	return m.data[filename], nil
}

func (m *MemoryBackend) Set(filename, value string) error {
	m.data[filename] = value
	return nil
}
