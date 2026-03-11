package cli_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/git"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestTelemetry_SandboxStart(t *testing.T) {
	t.Run("records sandbox.start for new sandbox", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "https://github.com/test/repo"
		setup.mockGit.MockGeneratePatchFile = git.PatchFile{}

		configPath := filepath.Join(setup.tmp, ".rwx", "sandbox.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("tasks:\n  - key: test\n"), 0o644))

		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			return &api.InitiateRunResult{
				RunID:  "run-new-123",
				RunURL: "https://cloud.rwx.com/runs/run-new-123",
			}, nil
		}

		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "token-123"}, nil
		}

		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{}, nil
		}

		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{}, nil
		}

		result, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, "run-new-123", result.RunID)

		events := setup.drainEvents()
		startEvents := findEvents(events, "sandbox.start")
		require.Len(t, startEvents, 1)
		require.Equal(t, false, startEvents[0].Props["reuse"])
	})

	t.Run("records sandbox.start with reuse=true for reattach", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-existing-456"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable: true,
				Polling:     api.PollingResult{Completed: false},
			}, nil
		}

		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "token-456"}, nil
		}

		result, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			RunID:      runID,
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, runID, result.RunID)

		events := setup.drainEvents()
		startEvent := findEvent(events, "sandbox.start")
		require.NotNil(t, startEvent)
		require.Equal(t, true, startEvent.Props["reuse"])
	})
}

func TestTelemetry_SessionCreatedAt(t *testing.T) {
	t.Run("sets CreatedAt on new sandbox session", func(t *testing.T) {
		setup := setupTest(t)

		before := time.Now().UTC().Add(-1 * time.Second)

		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "https://github.com/test/repo"
		setup.mockGit.MockGeneratePatchFile = git.PatchFile{}

		configPath := filepath.Join(setup.tmp, ".rwx", "sandbox.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("tasks:\n  - key: test\n"), 0o644))

		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			return &api.InitiateRunResult{RunID: "run-new-ts", RunURL: "url"}, nil
		}
		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "t"}, nil
		}
		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{}, nil
		}
		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{}, nil
		}

		_, err := setup.service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Json:       true,
		})
		require.NoError(t, err)

		// GetCurrentGitBranch uses real git, so in a temp dir with no repo it returns "detached"
		branch := cli.GetCurrentGitBranch(setup.tmp)
		storage, err := cli.LoadSandboxStorage()
		require.NoError(t, err)
		session, ok := storage.GetSession(setup.tmp, branch, ".rwx/sandbox.yml")
		require.True(t, ok)
		require.NotNil(t, session.CreatedAt)
		require.True(t, session.CreatedAt.After(before))
	})
}

func TestTelemetry_SSHConnect(t *testing.T) {
	t.Run("records ssh.connect success", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-ssh-ok"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        "192.168.1.1:22",
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}
		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}
		setup.mockSSH.MockExecuteCommandWithOutput = func(command string) (int, string, error) {
			return 0, "", nil
		}
		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo", "hello"},
			RunID:      runID,
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)

		events := setup.drainEvents()

		connectEvent := findEvent(events, "ssh.connect")
		require.NotNil(t, connectEvent)
		require.Equal(t, true, connectEvent.Props["success"])

		cmdEvent := findEvent(events, "ssh.command")
		require.NotNil(t, cmdEvent)
		require.Equal(t, 0, cmdEvent.Props["exit_code"])
		require.Equal(t, false, cmdEvent.Props["interactive"])
	})

	t.Run("records ssh.connect failure", func(t *testing.T) {
		setup := setupTest(t)

		runID := "run-ssh-fail"

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        "192.168.1.1:22",
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return &ssh.ExitError{}
		}

		_, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo"},
			RunID:      runID,
			Json:       true,
		})

		require.Error(t, err)

		events := setup.drainEvents()
		connectEvent := findEvent(events, "ssh.connect")
		require.NotNil(t, connectEvent)
		require.Equal(t, false, connectEvent.Props["success"])
	})
}

func TestTelemetry_DebugSSH(t *testing.T) {
	t.Run("records ssh.connect and ssh.command for debug session", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetDebugConnectionInfo = func(runId string) (api.DebugConnectionInfo, error) {
			return api.DebugConnectionInfo{
				Debuggable:     true,
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
				Address:        "192.168.1.1:22",
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}

		setup.mockSSH.MockInteractiveSession = func() error {
			return nil
		}

		err := setup.service.DebugTask(cli.DebugTaskConfig{DebugKey: "run-debug-123"})
		require.NoError(t, err)

		events := setup.drainEvents()

		connectEvent := findEvent(events, "ssh.connect")
		require.NotNil(t, connectEvent)
		require.Equal(t, true, connectEvent.Props["success"])

		cmdEvent := findEvent(events, "ssh.command")
		require.NotNil(t, cmdEvent)
		require.Equal(t, true, cmdEvent.Props["interactive"])
		require.Equal(t, 0, cmdEvent.Props["exit_code"])
	})
}

func TestTelemetry_SandboxExec(t *testing.T) {
	t.Run("records sandbox.exec with sync metrics", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        "192.168.1.1:22",
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}
		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}
		setup.mockSSH.MockExecuteCommandWithOutput = func(command string) (int, string, error) {
			return 0, "", nil
		}
		setup.mockSSH.MockExecuteCommandWithStdinAndCombinedOutput = func(command string, stdin io.Reader) (int, string, error) {
			return 0, "", nil
		}
		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return []byte("mock patch"), nil, nil
		}

		result, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"make", "test"},
			RunID:      "run-exec-123",
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)
		require.Equal(t, 0, result.ExitCode)

		events := setup.drainEvents()

		execEvent := findEvent(events, "sandbox.exec")
		require.NotNil(t, execEvent)
		require.Equal(t, 0, execEvent.Props["exit_code"])
		require.Contains(t, execEvent.Props, "duration_ms")
		require.Contains(t, execEvent.Props, "sync_push_ms")
		require.Contains(t, execEvent.Props, "sync_pull_ms")
		require.Contains(t, execEvent.Props, "push_patch_bytes")
		require.Contains(t, execEvent.Props, "pull_patch_bytes")
	})
}

func TestTelemetry_SandboxSyncPush(t *testing.T) {
	t.Run("records sandbox.sync_push with patch bytes", func(t *testing.T) {
		setup := setupTest(t)

		patchData := []byte("diff --git a/file.go b/file.go\n+new line\n")

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        "192.168.1.1:22",
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}
		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}
		setup.mockSSH.MockExecuteCommandWithOutput = func(command string) (int, string, error) {
			return 0, "", nil
		}
		setup.mockSSH.MockExecuteCommandWithStdinAndCombinedOutput = func(command string, stdin io.Reader) (int, string, error) {
			return 0, "", nil
		}
		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return patchData, nil, nil
		}

		_, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo"},
			RunID:      "run-sync-push",
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)

		events := setup.drainEvents()
		pushEvent := findEvent(events, "sandbox.sync_push")
		require.NotNil(t, pushEvent)
		require.Equal(t, len(patchData), pushEvent.Props["patch_bytes"])
		require.Equal(t, true, pushEvent.Props["success"])
	})
}

func TestTelemetry_SandboxSyncPull(t *testing.T) {
	t.Run("records sandbox.sync_pull on successful pull", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable:    true,
				Address:        "192.168.1.1:22",
				PrivateUserKey: sandboxPrivateTestKey,
				PublicHostKey:  sandboxPublicTestKey,
			}, nil
		}

		setup.mockSSH.MockConnect = func(addr string, _ ssh.ClientConfig) error {
			return nil
		}
		setup.mockSSH.MockExecuteCommand = func(cmd string) (int, error) {
			return 0, nil
		}
		setup.mockSSH.MockExecuteCommandWithOutput = func(command string) (int, string, error) {
			return 0, "", nil
		}
		setup.mockGit.MockGeneratePatch = func(pathspec []string) ([]byte, *git.LFSChangedFilesMetadata, error) {
			return nil, nil, nil
		}

		_, err := setup.service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Command:    []string{"echo"},
			RunID:      "run-sync-pull",
			Json:       true,
			Sync:       true,
		})

		require.NoError(t, err)

		events := setup.drainEvents()
		pullEvent := findEvent(events, "sandbox.sync_pull")
		require.NotNil(t, pullEvent)
		require.Equal(t, true, pullEvent.Props["success"])
		require.Contains(t, pullEvent.Props, "duration_ms")
		require.Contains(t, pullEvent.Props, "patch_bytes")
	})
}

func TestTelemetry_SandboxStop(t *testing.T) {
	t.Run("records sandbox.stop with lifetime and exec count", func(t *testing.T) {
		setup := setupTest(t)

		cwd := setup.tmp
		branch := cli.GetCurrentGitBranch(cwd)
		createdAt := time.Now().UTC().Add(-10 * time.Minute)

		storage, err := cli.LoadSandboxStorage()
		require.NoError(t, err)
		storage.SetSession(cwd, branch, ".rwx/sandbox.yml", cli.SandboxSession{
			RunID:      "run-stop-123",
			ConfigFile: ".rwx/sandbox.yml",
			CreatedAt:  &createdAt,
			ExecCount:  5,
		})
		require.NoError(t, storage.Save())

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			return api.SandboxConnectionInfo{
				Sandboxable: false,
				Polling:     api.PollingResult{Completed: true},
			}, nil
		}

		result, err := setup.service.StopSandbox(cli.StopSandboxConfig{
			Json: true,
		})

		require.NoError(t, err)
		require.Len(t, result.Stopped, 1)

		events := setup.drainEvents()
		stopEvent := findEvent(events, "sandbox.stop")
		require.NotNil(t, stopEvent)
		require.Equal(t, 5, stopEvent.Props["exec_count"])
		lifetimeS, ok := stopEvent.Props["lifetime_s"].(int64)
		require.True(t, ok)
		require.Greater(t, lifetimeS, int64(0))
	})
}

func TestTelemetry_SandboxReset(t *testing.T) {
	t.Run("records sandbox.reset", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "https://github.com/test/repo"
		setup.mockGit.MockGeneratePatchFile = git.PatchFile{}

		configPath := filepath.Join(setup.tmp, ".rwx", "sandbox.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("tasks:\n  - key: test\n"), 0o644))

		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			return &api.InitiateRunResult{
				RunID:  "run-reset-new",
				RunURL: "https://cloud.rwx.com/runs/run-reset-new",
			}, nil
		}

		setup.mockAPI.MockCreateSandboxToken = func(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
			return &api.CreateSandboxTokenResult{Token: "token"}, nil
		}

		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{}, nil
		}

		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{}, nil
		}

		result, err := setup.service.ResetSandbox(cli.ResetSandboxConfig{
			ConfigFile: ".rwx/sandbox.yml",
			Json:       true,
		})

		require.NoError(t, err)
		require.Equal(t, "run-reset-new", result.NewRunID)

		events := setup.drainEvents()
		resetEvent := findEvent(events, "sandbox.reset")
		require.NotNil(t, resetEvent)
	})
}

func TestTelemetry_SandboxList(t *testing.T) {
	t.Run("records sandbox.list with counts", func(t *testing.T) {
		setup := setupTest(t)

		cwd := setup.tmp
		branch := cli.GetCurrentGitBranch(cwd)

		// Seed one active and one expired session
		storage, err := cli.LoadSandboxStorage()
		require.NoError(t, err)
		storage.SetSession(cwd, branch, ".rwx/sandbox.yml", cli.SandboxSession{
			RunID:      "run-list-active",
			ConfigFile: ".rwx/sandbox.yml",
		})
		storage.SetSession(cwd, branch, ".rwx/other.yml", cli.SandboxSession{
			RunID:      "run-list-expired",
			ConfigFile: ".rwx/other.yml",
		})
		require.NoError(t, storage.Save())

		setup.mockAPI.MockListSandboxRuns = func() (*api.ListSandboxRunsResult, error) {
			return &api.ListSandboxRunsResult{
				Runs: []api.SandboxRunSummary{
					{ID: "run-list-active", RunURL: "url"},
				},
			}, nil
		}

		setup.mockAPI.MockGetSandboxConnectionInfo = func(id, token string) (api.SandboxConnectionInfo, error) {
			if id == "run-list-expired" {
				return api.SandboxConnectionInfo{
					Polling: api.PollingResult{Completed: true},
				}, nil
			}
			return api.SandboxConnectionInfo{
				Polling: api.PollingResult{Completed: false},
			}, nil
		}

		result, err := setup.service.ListSandboxes(cli.ListSandboxesConfig{Json: true})
		require.NoError(t, err)
		require.Len(t, result.Sandboxes, 1)

		events := setup.drainEvents()
		listEvent := findEvent(events, "sandbox.list")
		require.NotNil(t, listEvent)
		require.Equal(t, 1, listEvent.Props["total_count"])
		require.Equal(t, 1, listEvent.Props["active_count"])
		require.Equal(t, 1, listEvent.Props["pruned_count"])
	})
}

func TestTelemetry_RunInitiate(t *testing.T) {
	t.Run("records run.initiate", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockGit.MockGetBranch = "main"
		setup.mockGit.MockGetCommit = "abc123"
		setup.mockGit.MockGetOriginUrl = "https://github.com/test/repo"
		setup.mockGit.MockGeneratePatchFile = git.PatchFile{}

		configPath := filepath.Join(setup.tmp, ".rwx", "run.yml")
		require.NoError(t, os.WriteFile(configPath, []byte("tasks:\n  - key: test\n"), 0o644))

		setup.mockAPI.MockInitiateRun = func(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
			return &api.InitiateRunResult{
				RunID:  "run-init-123",
				RunURL: "https://cloud.rwx.com/runs/run-init-123",
			}, nil
		}

		setup.mockAPI.MockGetDefaultBase = func() (api.DefaultBaseResult, error) {
			return api.DefaultBaseResult{}, nil
		}

		setup.mockAPI.MockGetPackageVersions = func() (*api.PackageVersionsResult, error) {
			return &api.PackageVersionsResult{}, nil
		}

		result, err := setup.service.InitiateRun(cli.InitiateRunConfig{
			MintFilePath:  ".rwx/run.yml",
			Patchable:     true,
			TargetedTasks: []string{"test"},
		})

		require.NoError(t, err)
		require.Equal(t, "run-init-123", result.RunID)

		events := setup.drainEvents()
		initEvent := findEvent(events, "run.initiate")
		require.NotNil(t, initEvent)
		require.Equal(t, true, initEvent.Props["success"])
		require.Equal(t, true, initEvent.Props["has_targets"])
		require.Equal(t, false, initEvent.Props["has_init_params"])
		require.Contains(t, initEvent.Props, "duration_ms")
	})
}

func TestTelemetry_RunComplete(t *testing.T) {
	t.Run("records run.complete when polling shows completed", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				RunID:  "run-complete-123",
				RunURL: "https://cloud.rwx.com/runs/run-complete-123",
				Status: &api.RunStatus{Result: "succeeded"},
				Polling: api.PollingResult{
					Completed: true,
				},
			}, nil
		}

		result, err := setup.service.GetRunStatus(cli.GetRunStatusConfig{
			RunID: "run-complete-123",
			Wait:  true,
			Json:  true,
		})

		require.NoError(t, err)
		require.True(t, result.Completed)
		require.Equal(t, "succeeded", result.ResultStatus)

		events := setup.drainEvents()
		completeEvent := findEvent(events, "run.complete")
		require.NotNil(t, completeEvent)
		require.Equal(t, "succeeded", completeEvent.Props["result_status"])
		require.Equal(t, true, completeEvent.Props["wait"])
		require.Contains(t, completeEvent.Props, "wait_duration_ms")
	})

	t.Run("does not record run.complete when not completed", func(t *testing.T) {
		setup := setupTest(t)

		setup.mockAPI.MockRunStatus = func(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
			return api.RunStatusResult{
				RunID:  "run-pending-123",
				RunURL: "https://cloud.rwx.com/runs/run-pending-123",
				Status: &api.RunStatus{Result: "running"},
				Polling: api.PollingResult{
					Completed: false,
				},
			}, nil
		}

		result, err := setup.service.GetRunStatus(cli.GetRunStatusConfig{
			RunID: "run-pending-123",
			Wait:  false,
			Json:  true,
		})

		require.NoError(t, err)
		require.False(t, result.Completed)

		events := setup.drainEvents()
		completeEvent := findEvent(events, "run.complete")
		require.Nil(t, completeEvent)
	})
}
