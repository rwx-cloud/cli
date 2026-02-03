package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"

	"golang.org/x/crypto/ssh"
)

// Config types

type StartSandboxConfig struct {
	ConfigFile   string
	RunID        string
	RwxDirectory string
	Json         bool
	Wait         bool
}

type ExecSandboxConfig struct {
	ConfigFile   string
	Command      []string
	RunID        string
	RwxDirectory string
	Json         bool
	Sync         bool
}

type ListSandboxesConfig struct {
	Json bool
}

type StopSandboxConfig struct {
	RunID string
	All   bool
	Json  bool
}

type ResetSandboxConfig struct {
	ConfigFile   string
	RwxDirectory string
	Json         bool
	Wait         bool
}

// Result types

type StartSandboxResult struct {
	RunID      string
	RunURL     string
	ConfigFile string
}

type ExecSandboxResult struct {
	ExitCode int
	RunURL   string
}

type ListSandboxesResult struct {
	Sandboxes []SandboxInfo
}

type SandboxInfo struct {
	RunID      string
	Status     string
	ConfigFile string
	CWD        string
	Branch     string
}

type StopSandboxResult struct {
	Stopped []StoppedSandbox
}

type StoppedSandbox struct {
	RunID      string
	WasRunning bool
}

type ResetSandboxResult struct {
	OldRunID string
	NewRunID string
	RunURL   string
}

// Service methods

type CheckExistingSandboxResult struct {
	Exists     bool
	Active     bool
	RunID      string
	RunURL     string
	ConfigFile string
}

func (s Service) CheckExistingSandbox(configFile string) (*CheckExistingSandboxResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current directory")
	}
	branch := GetCurrentGitBranch(cwd)

	storage, err := LoadSandboxStorage()
	if err != nil {
		return &CheckExistingSandboxResult{Exists: false}, nil
	}

	session, found := storage.GetSession(cwd, branch, configFile)
	if !found {
		return &CheckExistingSandboxResult{Exists: false}, nil
	}

	// Check if the run is still active
	connInfo, err := s.APIClient.GetSandboxConnectionInfo(session.RunID)
	if err != nil {
		// Can't check status, treat as not active
		return &CheckExistingSandboxResult{
			Exists:     true,
			Active:     false,
			RunID:      session.RunID,
			ConfigFile: session.ConfigFile,
		}, nil
	}

	if connInfo.Polling.Completed {
		// Run has completed, not active
		return &CheckExistingSandboxResult{
			Exists:     true,
			Active:     false,
			RunID:      session.RunID,
			ConfigFile: session.ConfigFile,
		}, nil
	}

	runURL := fmt.Sprintf("https://cloud.rwx.com/mint/runs/%s", session.RunID)
	return &CheckExistingSandboxResult{
		Exists:     true,
		Active:     true,
		RunID:      session.RunID,
		RunURL:     runURL,
		ConfigFile: session.ConfigFile,
	}, nil
}

func (s Service) StartSandbox(cfg StartSandboxConfig) (*StartSandboxResult, error) {
	defer s.outputLatestVersionMessage()

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current directory")
	}
	branch := GetCurrentGitBranch(cwd)

	// If --id is provided, check if run is still active and reattach
	if cfg.RunID != "" {
		connInfo, err := s.APIClient.GetSandboxConnectionInfo(cfg.RunID)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get sandbox info for %s", cfg.RunID)
		}

		if connInfo.Polling.Completed {
			return nil, fmt.Errorf("Run '%s' is no longer active (timed out or cancelled).\nRun 'rwx sandbox start %s' to create a new sandbox.", cfg.RunID, cfg.ConfigFile)
		}

		// Only wait for sandbox to be ready if --wait flag is set
		if cfg.Wait && !connInfo.Sandboxable {
			if _, err := s.waitForSandboxReady(cfg.RunID, cfg.Json); err != nil {
				return nil, err
			}
		}

		// Store session if not already stored
		storage, err := LoadSandboxStorage()
		if err != nil {
			fmt.Fprintf(s.Stderr, "Warning: Unable to load sandbox sessions: %v\n", err)
		} else {
			if _, _, found := storage.FindByRunID(cfg.RunID); !found {
				storage.SetSession(cwd, branch, cfg.ConfigFile, SandboxSession{
					RunID:      cfg.RunID,
					ConfigFile: cfg.ConfigFile,
				})
				if err := storage.Save(); err != nil {
					fmt.Fprintf(s.Stderr, "Warning: Unable to save sandbox session: %v\n", err)
				}
			}
		}

		runURL := fmt.Sprintf("https://cloud.rwx.com/mint/runs/%s", cfg.RunID)
		if !cfg.Json {
			fmt.Fprintf(s.Stdout, "Attached to sandbox: %s\n%s\n", cfg.RunID, runURL)
		}

		return &StartSandboxResult{
			RunID:      cfg.RunID,
			RunURL:     runURL,
			ConfigFile: cfg.ConfigFile,
		}, nil
	}

	// Start a new sandbox run
	var finishSpinner func(string)
	if !cfg.Json {
		finishSpinner = SpinUntilDone("Starting sandbox...", s.StdoutIsTTY, s.Stdout)
	}

	runResult, err := s.InitiateRun(InitiateRunConfig{
		MintFilePath: cfg.ConfigFile,
		RwxDirectory: cfg.RwxDirectory,
		Json:         cfg.Json,
	})

	if err != nil {
		if finishSpinner != nil {
			finishSpinner("Failed to start sandbox")
		}
		return nil, err
	}

	if finishSpinner != nil {
		finishSpinner(fmt.Sprintf("Started sandbox: %s\n%s", runResult.RunId, runResult.RunURL))
	}

	// Build result now so we can return it even if waiting fails
	result := &StartSandboxResult{
		RunID:      runResult.RunId,
		RunURL:     runResult.RunURL,
		ConfigFile: cfg.ConfigFile,
	}

	// Store session (do this before waiting so it's saved even if waiting fails)
	storage, err := LoadSandboxStorage()
	if err != nil {
		fmt.Fprintf(s.Stderr, "Warning: Unable to load sandbox sessions: %v\n", err)
	} else {
		storage.SetSession(cwd, branch, cfg.ConfigFile, SandboxSession{
			RunID:      runResult.RunId,
			ConfigFile: cfg.ConfigFile,
		})
		if err := storage.Save(); err != nil {
			fmt.Fprintf(s.Stderr, "Warning: Unable to save sandbox session: %v\n", err)
		}
	}

	// Only wait for sandbox to be ready if --wait flag is set
	if cfg.Wait {
		_, err = s.waitForSandboxReady(runResult.RunId, cfg.Json)
		if err != nil {
			// Return result WITH error so caller can still use the URL
			return result, err
		}
	}

	return result, nil
}

func (s Service) ExecSandbox(cfg ExecSandboxConfig) (*ExecSandboxResult, error) {
	defer s.outputLatestVersionMessage()

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current directory")
	}
	branch := GetCurrentGitBranch(cwd)

	var runID string
	var configFile string

	// Sandbox selection priority:
	// 1. --id flag
	// 2. Find by CWD + git branch in storage
	// 3. Auto-create new sandbox

	if cfg.RunID != "" {
		// Use specified run ID directly - waitForSandboxReady will check if it's valid
		runID = cfg.RunID
		configFile = cfg.ConfigFile
	} else {
		// Try to find existing session
		storage, err := LoadSandboxStorage()
		if err != nil {
			fmt.Fprintf(s.Stderr, "Warning: Unable to load sandbox sessions: %v\n", err)
			storage = &SandboxStorage{Sandboxes: make(map[string]SandboxSession)}
		}

		var session *SandboxSession
		found := false

		if cfg.ConfigFile != "" {
			// Config file provided - look up specific session
			session, found = storage.GetSession(cwd, branch, cfg.ConfigFile)
			if found {
				// Check if session is still valid
				connInfo, err := s.APIClient.GetSandboxConnectionInfo(session.RunID)
				if err != nil {
					storage.DeleteSession(cwd, branch, cfg.ConfigFile)
					_ = storage.Save()
					found = false
				} else if connInfo.Polling.Completed {
					storage.DeleteSession(cwd, branch, cfg.ConfigFile)
					_ = storage.Save()
					found = false
				} else {
					runID = session.RunID
					configFile = session.ConfigFile
				}
			}
		} else {
			// No config file - find any session for cwd+branch
			sessions := storage.GetSessionsForCwdBranch(cwd, branch)

			// Filter to only active sessions
			var activeSessions []SandboxSession
			for _, sess := range sessions {
				connInfo, err := s.APIClient.GetSandboxConnectionInfo(sess.RunID)
				if err == nil && !connInfo.Polling.Completed {
					activeSessions = append(activeSessions, sess)
				} else {
					// Clean up expired session
					storage.DeleteSession(cwd, branch, sess.ConfigFile)
				}
			}
			_ = storage.Save()

			if len(activeSessions) == 1 {
				runID = activeSessions[0].RunID
				configFile = activeSessions[0].ConfigFile
				found = true
			} else if len(activeSessions) > 1 {
				return nil, fmt.Errorf("Multiple active sandboxes found for %s:%s.\nSpecify a config file to select one, or use --id to specify a run ID.", cwd, branch)
			}
		}

		if !found {
			// No existing sandbox - auto-create using provided config or default
			cfgFile := cfg.ConfigFile
			if cfgFile == "" {
				cfgFile = ".rwx/sandbox.yml"
			}

			startResult, err := s.StartSandbox(StartSandboxConfig{
				ConfigFile:   cfgFile,
				RwxDirectory: cfg.RwxDirectory,
				Json:         cfg.Json,
			})
			if err != nil {
				return nil, err
			}
			runID = startResult.RunID
			configFile = startResult.ConfigFile
		}
	}

	// Get connection info
	connInfo, err := s.waitForSandboxReady(runID, cfg.Json)
	if err != nil {
		return nil, err
	}

	// Connect via SSH
	err = s.connectSSH(connInfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to sandbox '%s': %v\nThe sandbox may have timed out. Run 'rwx sandbox reset %s' to restart.", runID, err, configFile)
	}
	defer s.SSHClient.Close()

	// Sync local changes to sandbox if enabled
	if cfg.Sync {
		if err := s.syncChangesToSandbox(cfg.Json); err != nil {
			return nil, errors.Wrap(err, "failed to sync changes to sandbox")
		}
	}

	// Execute command with proper shell escaping
	command := shellJoin(cfg.Command)
	exitCode, err := s.SSHClient.ExecuteCommand(command)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute command in sandbox")
	}

	runURL := fmt.Sprintf("https://cloud.rwx.com/mint/runs/%s", runID)
	return &ExecSandboxResult{ExitCode: exitCode, RunURL: runURL}, nil
}

func (s Service) ListSandboxes(cfg ListSandboxesConfig) (*ListSandboxesResult, error) {
	defer s.outputLatestVersionMessage()

	storage, err := LoadSandboxStorage()
	if err != nil {
		return nil, errors.Wrap(err, "unable to load sandbox sessions")
	}

	sandboxes := make([]SandboxInfo, 0, len(storage.Sandboxes))

	for key, session := range storage.AllSessions() {
		cwd, branch, _ := ParseSessionKey(key)

		status := "active"
		connInfo, err := s.APIClient.GetSandboxConnectionInfo(session.RunID)
		if err != nil {
			status = "unknown"
		} else if connInfo.Polling.Completed {
			status = "expired"
		}

		sandboxes = append(sandboxes, SandboxInfo{
			RunID:      session.RunID,
			Status:     status,
			ConfigFile: session.ConfigFile,
			CWD:        cwd,
			Branch:     branch,
		})
	}

	if !cfg.Json {
		if len(sandboxes) == 0 {
			fmt.Fprintln(s.Stdout, "No sandbox sessions found.")
		} else {
			fmt.Fprintf(s.Stdout, "%-40s %-10s %-25s %-40s %s\n", "RUN", "STATUS", "CONFIG", "CWD", "BRANCH")
			for _, sb := range sandboxes {
				cwdDisplay := sb.CWD
				if len(cwdDisplay) > 40 {
					cwdDisplay = "..." + cwdDisplay[len(cwdDisplay)-37:]
				}
				fmt.Fprintf(s.Stdout, "%-40s %-10s %-25s %-40s %s\n", sb.RunID, sb.Status, sb.ConfigFile, cwdDisplay, sb.Branch)
			}
		}
	}

	return &ListSandboxesResult{Sandboxes: sandboxes}, nil
}

func (s Service) StopSandbox(cfg StopSandboxConfig) (*StopSandboxResult, error) {
	defer s.outputLatestVersionMessage()

	storage, err := LoadSandboxStorage()
	if err != nil {
		return nil, errors.Wrap(err, "unable to load sandbox sessions")
	}

	var toStop []SandboxSession
	var keys []string

	if cfg.All {
		for key, session := range storage.AllSessions() {
			toStop = append(toStop, session)
			keys = append(keys, key)
		}
	} else if cfg.RunID != "" {
		session, key, found := storage.FindByRunID(cfg.RunID)
		if !found {
			return nil, fmt.Errorf("Sandbox with run ID '%s' not found in local storage.\nUse 'rwx sandbox list' to see available sandboxes.", cfg.RunID)
		}
		toStop = append(toStop, *session)
		keys = append(keys, key)
	} else {
		// Stop sandbox(es) for current CWD + branch
		cwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "unable to get current directory")
		}
		branch := GetCurrentGitBranch(cwd)

		sessions := storage.GetSessionsForCwdBranch(cwd, branch)
		if len(sessions) == 0 {
			return nil, fmt.Errorf("No sandbox found for %s:%s.\nUse 'rwx sandbox list' to see available sandboxes, or use --id to specify a run ID.", cwd, branch)
		}
		for _, session := range sessions {
			toStop = append(toStop, session)
			keys = append(keys, SessionKey(cwd, branch, session.ConfigFile))
		}
	}

	stopped := make([]StoppedSandbox, 0, len(toStop))

	for i, session := range toStop {
		wasRunning := false

		// Check if sandbox is still active and send stop command
		connInfo, err := s.APIClient.GetSandboxConnectionInfo(session.RunID)
		if err == nil && connInfo.Sandboxable {
			if err := s.connectSSH(&connInfo); err == nil {
				_, _ = s.SSHClient.ExecuteCommand("__rwx_sandbox_end__")
				s.SSHClient.Close()
				wasRunning = true
			}
		} else if err == nil && !connInfo.Polling.Completed {
			// Run is still active but not yet sandboxable
			wasRunning = true
		}

		// Remove from storage
		delete(storage.Sandboxes, keys[i])

		if !cfg.Json {
			if wasRunning {
				fmt.Fprintf(s.Stdout, "Stopped sandbox: %s\n", session.RunID)
			} else {
				fmt.Fprintf(s.Stdout, "Sandbox already stopped (run timed out): %s\n", session.RunID)
			}
		}

		stopped = append(stopped, StoppedSandbox{
			RunID:      session.RunID,
			WasRunning: wasRunning,
		})
	}

	if err := storage.Save(); err != nil {
		fmt.Fprintf(s.Stderr, "Warning: Unable to save sandbox sessions: %v\n", err)
	}

	return &StopSandboxResult{Stopped: stopped}, nil
}

func (s Service) ResetSandbox(cfg ResetSandboxConfig) (*ResetSandboxResult, error) {
	defer s.outputLatestVersionMessage()

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current directory")
	}
	branch := GetCurrentGitBranch(cwd)

	var oldRunID string

	// Check for existing sandbox with same config file
	storage, err := LoadSandboxStorage()
	if err != nil {
		fmt.Fprintf(s.Stderr, "Warning: Unable to load sandbox sessions: %v\n", err)
	} else {
		session, found := storage.GetSession(cwd, branch, cfg.ConfigFile)
		if found {
			oldRunID = session.RunID

			// Check if still running and stop it
			connInfo, err := s.APIClient.GetSandboxConnectionInfo(session.RunID)
			if err == nil && connInfo.Sandboxable {
				if err := s.connectSSH(&connInfo); err == nil {
					_, _ = s.SSHClient.ExecuteCommand("__rwx_sandbox_end__")
					s.SSHClient.Close()
				}
			}

			// Remove old session
			storage.DeleteSession(cwd, branch, cfg.ConfigFile)
			if err := storage.Save(); err != nil {
				fmt.Fprintf(s.Stderr, "Warning: Unable to save sandbox sessions: %v\n", err)
			}

			if !cfg.Json {
				fmt.Fprintf(s.Stdout, "Stopped old sandbox: %s\n", oldRunID)
			}
		}
	}

	// Start new sandbox
	startResult, err := s.StartSandbox(StartSandboxConfig{
		ConfigFile:   cfg.ConfigFile,
		RwxDirectory: cfg.RwxDirectory,
		Json:         cfg.Json,
		Wait:         cfg.Wait,
	})
	if err != nil {
		return nil, err
	}

	return &ResetSandboxResult{
		OldRunID: oldRunID,
		NewRunID: startResult.RunID,
		RunURL:   startResult.RunURL,
	}, nil
}

// Helper methods

func (s Service) waitForSandboxReady(runID string, jsonMode bool) (*api.SandboxConnectionInfo, error) {
	var stopSpinner func()
	if !jsonMode {
		stopSpinner = Spin("Waiting for sandbox to be ready...", s.StdoutIsTTY, s.Stdout)
		defer stopSpinner()
	}

	for {
		connInfo, err := s.APIClient.GetSandboxConnectionInfo(runID)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get sandbox connection info")
		}

		if connInfo.Sandboxable {
			return &connInfo, nil
		}

		if connInfo.Polling.Completed {
			return nil, fmt.Errorf("Sandbox run '%s' completed before becoming ready", runID)
		}

		// Use backoff from server, or default to 2 seconds
		backoffMs := 2000
		if connInfo.Polling.BackoffMs != nil {
			backoffMs = *connInfo.Polling.BackoffMs
		}
		time.Sleep(time.Duration(backoffMs) * time.Millisecond)
	}
}

func (s Service) connectSSH(connInfo *api.SandboxConnectionInfo) error {
	privateUserKey, err := ssh.ParsePrivateKey([]byte(connInfo.PrivateUserKey))
	if err != nil {
		return errors.Wrap(err, "unable to parse key material retrieved from Cloud API")
	}

	rawPublicHostKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(connInfo.PublicHostKey))
	if err != nil {
		return errors.Wrap(err, "unable to parse host key retrieved from Cloud API")
	}

	publicHostKey, err := ssh.ParsePublicKey(rawPublicHostKey.Marshal())
	if err != nil {
		return errors.Wrap(err, "unable to parse host key retrieved from Cloud API")
	}

	sshConfig := ssh.ClientConfig{
		User:            "mint-cli",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(privateUserKey)},
		HostKeyCallback: ssh.FixedHostKey(publicHostKey),
	}

	if err = s.SSHClient.Connect(connInfo.Address, sshConfig); err != nil {
		return errors.Wrap(err, "unable to establish SSH connection to remote host")
	}

	return nil
}

func (s Service) syncChangesToSandbox(jsonMode bool) error {
	patch, lfsFiles, err := s.GitClient.GeneratePatch(nil)
	if err != nil {
		return errors.Wrap(err, "failed to generate patch")
	}

	// Warn about LFS files
	if lfsFiles != nil && lfsFiles.Count > 0 {
		if !jsonMode {
			fmt.Fprintf(s.Stderr, "Warning: %d LFS file(s) changed locally and cannot be synced.\n", lfsFiles.Count)
		}
		return nil
	}

	// Get COMMIT_SHA from sandbox environment to reset to the correct base commit
	exitCode, commitSHA, err := s.SSHClient.ExecuteCommandWithOutput("echo $COMMIT_SHA")
	if err != nil {
		return errors.Wrap(err, "failed to get COMMIT_SHA from sandbox")
	}
	if exitCode != 0 {
		return fmt.Errorf("failed to get COMMIT_SHA from sandbox (exit code %d)", exitCode)
	}

	commitSHA = strings.TrimSpace(commitSHA)
	if commitSHA == "" {
		return fmt.Errorf("COMMIT_SHA environment variable is not set in the sandbox. Add `env: COMMIT_SHA: ${{ init.commit }}` to your sandbox task definition")
	}

	// Reset working directory to the base commit (clears any previous patches)
	exitCode, err = s.SSHClient.ExecuteCommand(fmt.Sprintf("{ /usr/bin/git reset --hard %s && /usr/bin/git clean -fd; } > /dev/null 2>&1", commitSHA))
	if err != nil {
		return errors.Wrap(err, "failed to reset sandbox working directory")
	}
	if exitCode != 0 {
		return fmt.Errorf("git reset failed with exit code %d", exitCode)
	}

	// Skip applying patch if no changes
	if len(patch) == 0 {
		return nil
	}

	// Apply patch on remote (use full path since sandbox session may have minimal PATH)
	exitCode, err = s.SSHClient.ExecuteCommandWithStdin("/usr/bin/git apply --allow-empty - > /dev/null 2>&1", bytes.NewReader(patch))
	if err != nil {
		return errors.Wrap(err, "failed to apply patch on sandbox")
	}
	if exitCode == 127 {
		return fmt.Errorf("git is not installed in the sandbox. Add a task that installs git before the sandbox task")
	}
	if exitCode != 0 {
		return fmt.Errorf("git apply failed with exit code %d", exitCode)
	}

	return nil
}

// shellJoin joins command arguments with proper shell escaping.
// Each argument is single-quoted to preserve its literal value,
// with internal single quotes properly escaped.
func shellJoin(args []string) string {
	escaped := make([]string, len(args))
	for i, arg := range args {
		escaped[i] = shellEscape(arg)
	}
	return strings.Join(escaped, " ")
}

// shellEscape escapes a single argument for safe shell execution.
// Returns the argument single-quoted with internal single quotes escaped.
func shellEscape(arg string) string {
	// If the argument is simple (no special chars), return as-is for readability
	if isSimpleArg(arg) {
		return arg
	}
	// Single-quote the argument, escaping any internal single quotes
	// by ending the quoted section, adding an escaped quote, and restarting
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

// isSimpleArg returns true if the argument contains no shell metacharacters
// and can be passed without quoting.
func isSimpleArg(arg string) bool {
	if arg == "" {
		return false
	}
	for _, c := range arg {
		if !isSimpleChar(c) {
			return false
		}
	}
	return true
}

// isSimpleChar returns true for characters that don't require shell escaping.
func isSimpleChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '/' || c == ':'
}
