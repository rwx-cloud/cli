package cli_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_DownloadArtifact(t *testing.T) {
	t.Run("when the artifact is not found", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			require.Equal(t, "task-123", taskId)
			require.Equal(t, "my-artifact", artifactKey)
			return api.ArtifactDownloadRequestResult{}, api.ErrNotFound
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-123",
			ArtifactKey: "my-artifact",
			OutputDir:   s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "Artifact my-artifact for task task-123 not found")
	})

	t.Run("when GetArtifactDownloadRequest fails with other error", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{}, errors.New("network error")
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-123",
			ArtifactKey: "my-artifact",
			OutputDir:   s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to fetch artifact download request")
		require.Contains(t, err.Error(), "network error")
	})

	t.Run("when DownloadArtifact fails", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-123-my-artifact.tar",
				Kind:     "file",
				Key:      "my-artifact",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			require.Equal(t, "https://example.com/artifact", request.URL)
			require.Equal(t, "task-123-my-artifact.tar", request.Filename)
			return nil, errors.New("download failed")
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-123",
			ArtifactKey: "my-artifact",
			OutputDir:   s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to download artifact")
		require.Contains(t, err.Error(), "download failed")
		require.Contains(t, s.mockStderr.String(), "Downloading artifact...")
	})

	t.Run("when validation fails - missing task ID", func(t *testing.T) {
		s := setupTest(t)

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "",
			ArtifactKey: "my-artifact",
			OutputDir:   s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		require.Contains(t, err.Error(), "task ID must be provided")
	})

	t.Run("when validation fails - missing artifact key", func(t *testing.T) {
		s := setupTest(t)

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-123",
			ArtifactKey: "",
			OutputDir:   s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		require.Contains(t, err.Error(), "artifact key must be provided")
	})

	t.Run("when validation fails - both output-dir and output-file set", func(t *testing.T) {
		s := setupTest(t)

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-123",
			ArtifactKey: "my-artifact",
			OutputDir:   s.tmp,
			OutputFile:  filepath.Join(s.tmp, "custom.txt"),
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		require.Contains(t, err.Error(), "output-dir and output-file cannot be used together")
	})

	t.Run("when download succeeds with file artifact - always extracts", func(t *testing.T) {
		s := setupTest(t)

		fileContent := []byte("artifact file content")
		tarBytes := createTestTar(t, map[string][]byte{
			"myfile.txt": fileContent,
		})

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			require.Equal(t, "task-123", taskId)
			require.Equal(t, "my-file", artifactKey)
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-123-my-file.tar",
				Kind:     "file",
				Key:      "my-file",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return tarBytes, nil
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-123",
			ArtifactKey: "my-file",
			OutputDir:   s.tmp,
			AutoExtract: false, // Should extract anyway for files
		})

		require.NoError(t, err)
		extractDir := filepath.Join(s.tmp, "task-123-my-file")
		expectedPath := filepath.Join(extractDir, "myfile.txt")
		require.FileExists(t, expectedPath)

		actualContents, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		require.Equal(t, fileContent, actualContents)

		output := s.mockStdout.String()
		require.Contains(t, output, "Artifact downloaded to")
		require.Contains(t, output, "myfile.txt")
		require.Contains(t, s.mockStderr.String(), "Downloading artifact...")
	})

	t.Run("when download succeeds with directory artifact and auto-extract false - saves tar", func(t *testing.T) {
		s := setupTest(t)

		tarBytes := createTestTar(t, map[string][]byte{
			"file1.txt": []byte("content 1"),
			"file2.txt": []byte("content 2"),
		})

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-456-my-dir.tar",
				Kind:     "directory",
				Key:      "my-dir",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return tarBytes, nil
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-456",
			ArtifactKey: "my-dir",
			OutputDir:   s.tmp,
			AutoExtract: false,
		})

		require.NoError(t, err)
		expectedPath := filepath.Join(s.tmp, "task-456-my-dir.tar")
		require.FileExists(t, expectedPath)

		actualContents, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		require.Equal(t, tarBytes, actualContents)

		output := s.mockStdout.String()
		require.Contains(t, output, "Artifact downloaded to")
		require.Contains(t, output, "task-456-my-dir.tar")
	})

	t.Run("when download succeeds with directory artifact and auto-extract true - extracts", func(t *testing.T) {
		s := setupTest(t)

		tarBytes := createTestTar(t, map[string][]byte{
			"file1.txt":        []byte("content 1"),
			"file2.txt":        []byte("content 2"),
			"subdir/file3.txt": []byte("content 3"),
		})

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-789-my-dir.tar",
				Kind:     "directory",
				Key:      "my-dir",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return tarBytes, nil
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-789",
			ArtifactKey: "my-dir",
			OutputDir:   s.tmp,
			AutoExtract: true,
		})

		require.NoError(t, err)
		extractDir := filepath.Join(s.tmp, "task-789-my-dir")
		require.FileExists(t, filepath.Join(extractDir, "file1.txt"))
		require.FileExists(t, filepath.Join(extractDir, "file2.txt"))
		require.FileExists(t, filepath.Join(extractDir, "subdir", "file3.txt"))

		content1, err := os.ReadFile(filepath.Join(extractDir, "file1.txt"))
		require.NoError(t, err)
		require.Equal(t, []byte("content 1"), content1)

		output := s.mockStdout.String()
		require.Contains(t, output, "Extracted 3 file(s)")
		require.Contains(t, output, "file1.txt")
		require.Contains(t, output, "file2.txt")
		require.Contains(t, output, "subdir/file3.txt")
	})

	t.Run("when download succeeds with OutputFile specified for file artifact", func(t *testing.T) {
		s := setupTest(t)

		fileContent := []byte("custom file content")
		tarBytes := createTestTar(t, map[string][]byte{
			"original.txt": fileContent,
		})

		customOutputFile := filepath.Join(s.tmp, "custom", "renamed.txt")
		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-999-my-file.tar",
				Kind:     "file",
				Key:      "my-file",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return tarBytes, nil
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-999",
			ArtifactKey: "my-file",
			OutputFile:  customOutputFile,
		})

		require.NoError(t, err)
		require.FileExists(t, customOutputFile)

		actualContents, err := os.ReadFile(customOutputFile)
		require.NoError(t, err)
		require.Equal(t, fileContent, actualContents)

		output := s.mockStdout.String()
		require.Contains(t, output, "Artifact downloaded to")
		require.Contains(t, output, "renamed.txt")
	})

	t.Run("when download succeeds with JSON output - single file", func(t *testing.T) {
		s := setupTest(t)

		tarBytes := createTestTar(t, map[string][]byte{
			"result.json": []byte(`{"status":"success"}`),
		})

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-111-result.tar",
				Kind:     "file",
				Key:      "result",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return tarBytes, nil
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-111",
			ArtifactKey: "result",
			OutputDir:   s.tmp,
			Json:        true,
		})

		require.NoError(t, err)
		output := s.mockStdout.String()
		require.Contains(t, output, `"OutputFiles"`)
		require.Contains(t, output, "result.json")
		require.NotContains(t, output, "Artifact downloaded to")
	})

	t.Run("when download succeeds with JSON output and auto-extract - directory", func(t *testing.T) {
		s := setupTest(t)

		tarBytes := createTestTar(t, map[string][]byte{
			"file1.txt": []byte("content 1"),
			"file2.txt": []byte("content 2"),
		})

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-222-my-dir.tar",
				Kind:     "directory",
				Key:      "my-dir",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return tarBytes, nil
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-222",
			ArtifactKey: "my-dir",
			OutputDir:   s.tmp,
			AutoExtract: true,
			Json:        true,
		})

		require.NoError(t, err)
		output := s.mockStdout.String()
		require.Contains(t, output, `"OutputFiles"`)
		require.Contains(t, output, "file1.txt")
		require.Contains(t, output, "file2.txt")
		require.NotContains(t, output, "Extracted")
		require.NotContains(t, output, "Artifact downloaded")
	})

	t.Run("when tar contains ./ directory entry", func(t *testing.T) {
		s := setupTest(t)

		// Create tar with ./ entry (common in some tar files)
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)

		// Add ./ directory entry
		err := tw.WriteHeader(&tar.Header{
			Name:     "./",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		})
		require.NoError(t, err)

		// Add a regular file
		content := []byte("file content")
		err = tw.WriteHeader(&tar.Header{
			Name:     "./file.txt",
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
			Mode:     0644,
		})
		require.NoError(t, err)
		_, err = tw.Write(content)
		require.NoError(t, err)

		err = tw.Close()
		require.NoError(t, err)

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "task-444-dotslash.tar",
				Kind:     "directory",
				Key:      "dotslash",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return buf.Bytes(), nil
		}

		_, err = s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-444",
			ArtifactKey: "dotslash",
			OutputDir:   s.tmp,
			AutoExtract: true,
		})

		require.NoError(t, err)
		extractDir := filepath.Join(s.tmp, "task-444-dotslash")
		require.FileExists(t, filepath.Join(extractDir, "file.txt"))

		actualContents, err := os.ReadFile(filepath.Join(extractDir, "file.txt"))
		require.NoError(t, err)
		require.Equal(t, content, actualContents)
	})

	t.Run("when filename contains path traversal attempt - sanitizes directory name", func(t *testing.T) {
		s := setupTest(t)

		tarBytes := createTestTar(t, map[string][]byte{
			"safe.txt": []byte("safe content"),
		})

		s.mockAPI.MockGetArtifactDownloadRequest = func(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
			return api.ArtifactDownloadRequestResult{
				URL:      "https://example.com/artifact",
				Filename: "../../etc/evil.tar", // Path traversal attempt
				Kind:     "file",
				Key:      "evil",
			}, nil
		}

		s.mockAPI.MockDownloadArtifact = func(request api.ArtifactDownloadRequestResult) ([]byte, error) {
			return tarBytes, nil
		}

		_, err := s.service.DownloadArtifact(cli.DownloadArtifactConfig{
			TaskID:      "task-999",
			ArtifactKey: "evil",
			OutputDir:   s.tmp,
		})

		require.NoError(t, err)
		// Should extract to safe sanitized directory name "evil" instead of "../../etc/evil"
		extractDir := filepath.Join(s.tmp, "evil")
		require.FileExists(t, filepath.Join(extractDir, "safe.txt"))

		actualContents, err := os.ReadFile(filepath.Join(extractDir, "safe.txt"))
		require.NoError(t, err)
		require.Equal(t, []byte("safe content"), actualContents)

		// Verify file was NOT created outside the temp directory
		evilPath := filepath.Join(s.tmp, "..", "..", "etc", "evil", "safe.txt")
		require.NoFileExists(t, evilPath)
	})
}

func createTestTar(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for name, content := range files {
		// Create directory entries if needed
		if dir := filepath.Dir(name); dir != "." {
			err := tw.WriteHeader(&tar.Header{
				Name:     dir + "/",
				Typeflag: tar.TypeDir,
				Mode:     0755,
			})
			require.NoError(t, err)
		}

		// Create file entry
		err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
			Mode:     0644,
		})
		require.NoError(t, err)

		_, err = tw.Write(content)
		require.NoError(t, err)
	}

	err := tw.Close()
	require.NoError(t, err)

	return buf.Bytes()
}
