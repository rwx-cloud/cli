package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_DownloadLogs(t *testing.T) {
	t.Run("when the task is not found", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			require.Equal(t, "task-123", taskId)
			return api.LogDownloadRequestResult{}, api.ErrNotFound
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "Task task-123 not found")
	})

	t.Run("when GetLogDownloadRequest fails with other error", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			return api.LogDownloadRequestResult{}, errors.New("network error")
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to fetch log archive request")
		require.Contains(t, err.Error(), "network error")
	})

	t.Run("when DownloadLogs fails", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			return api.LogDownloadRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "logs.log",
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogDownloadRequestResult) ([]byte, error) {
			require.Equal(t, "https://example.com/logs", request.URL)
			require.Equal(t, "jwt-token", request.Token)
			require.Equal(t, "logs.log", request.Filename)
			return nil, errors.New("download failed")
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to download logs")
		require.Contains(t, err.Error(), "download failed")
		require.Contains(t, s.mockStderr.String(), "Downloading logs...")
	})

	t.Run("when output directory does not exist, it is created", func(t *testing.T) {
		s := setupTest(t)

		logContents := []byte("log file contents")
		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			return api.LogDownloadRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "logs.log",
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogDownloadRequestResult) ([]byte, error) {
			return logContents, nil
		}

		nestedDir := filepath.Join(s.tmp, "nonexistent", "subdir")
		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: nestedDir,
		})

		require.NoError(t, err)
		expectedPath := filepath.Join(nestedDir, "logs.log")
		require.FileExists(t, expectedPath)

		actualContents, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		require.Equal(t, logContents, actualContents)
		require.Contains(t, s.mockStderr.String(), "Downloading logs...")
	})

	t.Run("when download succeeds with single log file (no Contents)", func(t *testing.T) {
		s := setupTest(t)

		logContents := []byte("2024-01-01 12:00:00 INFO Starting task\n2024-01-01 12:00:01 INFO Task completed\n")
		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			require.Equal(t, "task-123", taskId)
			return api.LogDownloadRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "task-123-logs.log",
				Contents: nil, // No contents = single log file
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogDownloadRequestResult) ([]byte, error) {
			require.Equal(t, "https://example.com/logs", request.URL)
			require.Equal(t, "jwt-token", request.Token)
			require.Equal(t, "task-123-logs.log", request.Filename)
			require.Nil(t, request.Contents)
			return logContents, nil
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: s.tmp,
		})

		require.NoError(t, err)
		expectedPath := filepath.Join(s.tmp, "task-123-logs.log")
		require.FileExists(t, expectedPath)

		actualContents, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		require.Equal(t, logContents, actualContents)

		output := s.mockStdout.String()
		require.Contains(t, output, "Logs downloaded to")
		require.Contains(t, output, "task-123-logs.log")
		require.Contains(t, s.mockStderr.String(), "Downloading logs...")
	})

	t.Run("when download succeeds with zip file (with Contents)", func(t *testing.T) {
		s := setupTest(t)

		zipContents := []byte("PK\x03\x04\x14\x00\x08\x00\x08\x00")
		contents := `{"key":"value"}`
		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			require.Equal(t, "task-456", taskId)
			return api.LogDownloadRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "task-456-logs.zip",
				Contents: &contents, // Contents present = zip file
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogDownloadRequestResult) ([]byte, error) {
			require.Equal(t, "https://example.com/logs", request.URL)
			require.Equal(t, "jwt-token", request.Token)
			require.Equal(t, "task-456-logs.zip", request.Filename)
			require.NotNil(t, request.Contents)
			require.Equal(t, contents, *request.Contents)
			return zipContents, nil
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-456",
			OutputDir: s.tmp,
		})

		require.NoError(t, err)
		expectedPath := filepath.Join(s.tmp, "task-456-logs.zip")
		require.FileExists(t, expectedPath)

		actualContents, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		require.Equal(t, zipContents, actualContents)

		output := s.mockStdout.String()
		require.Contains(t, output, "Logs downloaded to")
		require.Contains(t, output, "task-456-logs.zip")
		require.Contains(t, s.mockStderr.String(), "Downloading logs...")
	})

	t.Run("when validation fails - missing task ID", func(t *testing.T) {
		s := setupTest(t)

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "",
			OutputDir: s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		require.Contains(t, err.Error(), "task ID must be provided")
	})

	t.Run("when validation fails - both output-dir and output-file set", func(t *testing.T) {
		s := setupTest(t)

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:     "task-123",
			OutputDir:  s.tmp,
			OutputFile: filepath.Join(s.tmp, "custom.log"),
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		require.Contains(t, err.Error(), "output-dir and output-file cannot be used together")
	})

	t.Run("when download succeeds with OutputFile specified", func(t *testing.T) {
		s := setupTest(t)

		logContents := []byte("2024-01-01 12:00:00 INFO Starting task\n2024-01-01 12:00:01 INFO Task completed\n")
		customOutputFile := filepath.Join(s.tmp, "custom", "my-logs.log")
		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			require.Equal(t, "task-789", taskId)
			return api.LogDownloadRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "task-789-logs.log",
				Contents: nil,
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogDownloadRequestResult) ([]byte, error) {
			return logContents, nil
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:     "task-789",
			OutputFile: customOutputFile,
		})

		require.NoError(t, err)
		require.FileExists(t, customOutputFile)

		actualContents, err := os.ReadFile(customOutputFile)
		require.NoError(t, err)
		require.Equal(t, logContents, actualContents)

		output := s.mockStdout.String()
		require.Contains(t, output, "Logs downloaded to")
		require.Contains(t, output, "my-logs.log")
		require.Contains(t, s.mockStderr.String(), "Downloading logs...")
	})

	t.Run("when download succeeds with OutputFile in nested directory", func(t *testing.T) {
		s := setupTest(t)

		logContents := []byte("log content")
		nestedOutputFile := filepath.Join(s.tmp, "nested", "deep", "path", "logs.log")
		s.mockAPI.MockGetLogDownloadRequest = func(taskId string) (api.LogDownloadRequestResult, error) {
			return api.LogDownloadRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "task-999-logs.log",
				Contents: nil,
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogDownloadRequestResult) ([]byte, error) {
			return logContents, nil
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:     "task-999",
			OutputFile: nestedOutputFile,
		})

		require.NoError(t, err)
		require.FileExists(t, nestedOutputFile)
	})
}
