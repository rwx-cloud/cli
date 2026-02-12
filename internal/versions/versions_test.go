package versions

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rwx-cloud/cli/internal/config"
	"github.com/stretchr/testify/require"
)

type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return m.response, m.err
}

func TestVersionInterceptor(t *testing.T) {
	t.Run("parses version from response header", func(t *testing.T) {
		versionHolder.latestVersion = EmptyVersion

		backend := NewMemoryBackend()
		mock := &mockRoundTripper{
			response: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
				Header:     http.Header{latestVersionHeader: []string{"1000000.0.0"}},
			},
		}

		rt := NewRoundTripper(mock, backend)
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.NoError(t, err)
		require.Equal(t, "1000000.0.0", GetCliLatestVersion().String())
		require.True(t, NewVersionAvailable())

		savedVersion, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "1000000.0.0", savedVersion)
	})

	t.Run("does not update version when header is missing", func(t *testing.T) {
		versionHolder.latestVersion = EmptyVersion

		backend := NewMemoryBackend()
		mock := &mockRoundTripper{
			response: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
				Header:     http.Header{},
			},
		}

		rt := NewRoundTripper(mock, backend)
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.NoError(t, err)
		require.True(t, GetCliLatestVersion().Equal(EmptyVersion))

		savedVersion, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "", savedVersion)
	})

	t.Run("does not update version when request errors", func(t *testing.T) {
		versionHolder.latestVersion = EmptyVersion

		backend := NewMemoryBackend()
		mock := &mockRoundTripper{
			response: nil,
			err:      io.EOF,
		}

		rt := NewRoundTripper(mock, backend)
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.Error(t, err)
		require.True(t, GetCliLatestVersion().Equal(EmptyVersion))

		savedVersion, err := backend.Get()
		require.NoError(t, err)
		require.Equal(t, "", savedVersion)
	})

	t.Run("works with nil backend", func(t *testing.T) {
		versionHolder.latestVersion = EmptyVersion

		mock := &mockRoundTripper{
			response: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
				Header:     http.Header{latestVersionHeader: []string{"2.0.0"}},
			},
		}

		rt := NewRoundTripper(mock, nil)
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := rt.RoundTrip(req)

		require.NoError(t, err)
		require.Equal(t, "2.0.0", GetCliLatestVersion().String())
	})
}

func TestLoadLatestVersionFromFile(t *testing.T) {
	t.Run("when config backend is nil", func(t *testing.T) {
		LoadLatestVersionFromFile(nil)
	})

	t.Run("when the file does not exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)

		LoadLatestVersionFromFile(backend)
	})

	t.Run("when the file contains a valid version", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		err = os.WriteFile(filepath.Join(tmpDir, latestVersionFilename), []byte("1.2.3"), 0o644)
		require.NoError(t, err)

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)
		versionHolder.latestVersion = EmptyVersion

		LoadLatestVersionFromFile(backend)
		require.Equal(t, "1.2.3", GetCliLatestVersion().String())
	})

	t.Run("when the file contains whitespace around the version", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		err = os.WriteFile(filepath.Join(tmpDir, latestVersionFilename), []byte("  2.0.0\n"), 0o644)
		require.NoError(t, err)

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)
		versionHolder.latestVersion = EmptyVersion

		LoadLatestVersionFromFile(backend)
		require.Equal(t, "2.0.0", GetCliLatestVersion().String())
	})

	t.Run("when the file is empty", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		err = os.WriteFile(filepath.Join(tmpDir, latestVersionFilename), []byte(""), 0o644)
		require.NoError(t, err)

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)
		versionHolder.latestVersion = EmptyVersion

		LoadLatestVersionFromFile(backend)
		require.True(t, GetCliLatestVersion().Equal(EmptyVersion))
	})

	t.Run("when the file contains an invalid version", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		err = os.WriteFile(filepath.Join(tmpDir, latestVersionFilename), []byte("not-a-version"), 0o644)
		require.NoError(t, err)

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)
		versionHolder.latestVersion = EmptyVersion

		LoadLatestVersionFromFile(backend)
		require.True(t, GetCliLatestVersion().Equal(EmptyVersion))
	})
}

func TestSaveLatestVersionToFile(t *testing.T) {
	t.Run("when config backend is nil", func(t *testing.T) {
		SaveLatestVersionToFile(nil)
	})

	t.Run("when latest version is empty", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)
		versionHolder.latestVersion = EmptyVersion

		SaveLatestVersionToFile(backend)

		_, err = os.Stat(filepath.Join(tmpDir, latestVersionFilename))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("when latest version is set", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)
		err = SetCliLatestVersion("3.4.5")
		require.NoError(t, err)

		SaveLatestVersionToFile(backend)

		contents, err := os.ReadFile(filepath.Join(tmpDir, latestVersionFilename))
		require.NoError(t, err)
		require.Equal(t, "3.4.5", string(contents))
	})

	t.Run("overwrites garbage file with valid version", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "versions-test")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })

		err = os.WriteFile(filepath.Join(tmpDir, latestVersionFilename), []byte("garbage-data"), 0o644)
		require.NoError(t, err)

		configBackend, err := config.NewFileBackend([]string{tmpDir})
		require.NoError(t, err)
		backend := NewFileBackend(configBackend)
		err = SetCliLatestVersion("5.0.0")
		require.NoError(t, err)

		SaveLatestVersionToFile(backend)

		contents, err := os.ReadFile(filepath.Join(tmpDir, latestVersionFilename))
		require.NoError(t, err)
		require.Equal(t, "5.0.0", string(contents))
	})
}
