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

		s.mockAPI.MockGetLogArchiveRequest = func(taskId string) (api.LogArchiveRequestResult, error) {
			require.Equal(t, "task-123", taskId)
			return api.LogArchiveRequestResult{}, api.ErrNotFound
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "Task task-123 not found")
	})

	t.Run("when GetLogArchiveRequest fails with other error", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetLogArchiveRequest = func(taskId string) (api.LogArchiveRequestResult, error) {
			return api.LogArchiveRequestResult{}, errors.New("network error")
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

		s.mockAPI.MockGetLogArchiveRequest = func(taskId string) (api.LogArchiveRequestResult, error) {
			return api.LogArchiveRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "logs.zip",
				Contents: "contents-json",
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogArchiveRequestResult) ([]byte, error) {
			require.Equal(t, "https://example.com/logs", request.URL)
			require.Equal(t, "jwt-token", request.Token)
			require.Equal(t, "logs.zip", request.Filename)
			require.Equal(t, "contents-json", request.Contents)
			return nil, errors.New("download failed")
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to download logs")
		require.Contains(t, err.Error(), "download failed")
	})

	t.Run("when writing file fails", func(t *testing.T) {
		s := setupTest(t)

		s.mockAPI.MockGetLogArchiveRequest = func(taskId string) (api.LogArchiveRequestResult, error) {
			return api.LogArchiveRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "logs.zip",
				Contents: "contents-json",
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogArchiveRequestResult) ([]byte, error) {
			return []byte("zip file contents"), nil
		}

		invalidDir := filepath.Join(s.tmp, "nonexistent", "subdir")
		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: invalidDir,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to write log file")
	})

	t.Run("when download succeeds", func(t *testing.T) {
		s := setupTest(t)

		zipContents := []byte("PK\x03\x04\x14\x00\x08\x00\x08\x00")
		s.mockAPI.MockGetLogArchiveRequest = func(taskId string) (api.LogArchiveRequestResult, error) {
			require.Equal(t, "task-123", taskId)
			return api.LogArchiveRequestResult{
				URL:      "https://example.com/logs",
				Token:    "jwt-token",
				Filename: "task-123-logs.zip",
				Contents: "contents-json",
			}, nil
		}

		s.mockAPI.MockDownloadLogs = func(request api.LogArchiveRequestResult) ([]byte, error) {
			require.Equal(t, "https://example.com/logs", request.URL)
			require.Equal(t, "jwt-token", request.Token)
			require.Equal(t, "task-123-logs.zip", request.Filename)
			require.Equal(t, "contents-json", request.Contents)
			return zipContents, nil
		}

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "task-123",
			OutputDir: s.tmp,
		})

		require.NoError(t, err)

		expectedPath := filepath.Join(s.tmp, "task-123-logs.zip")
		require.FileExists(t, expectedPath)

		actualContents, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		require.Equal(t, zipContents, actualContents)

		output := s.mockStdout.String()
		require.Contains(t, output, "Logs downloaded to")
		require.Contains(t, output, "task-123-logs.zip")
	})

	t.Run("when validation fails", func(t *testing.T) {
		s := setupTest(t)

		err := s.service.DownloadLogs(cli.DownloadLogsConfig{
			TaskID:    "",
			OutputDir: s.tmp,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		require.Contains(t, err.Error(), "task ID must be provided")
	})
}
