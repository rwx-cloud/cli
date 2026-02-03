package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

type GetSandboxInitTemplateConfig struct {
	Json bool
}

type GetSandboxInitTemplateResult struct {
	Template string
}

type PullSandboxConfig struct {
	ConfigFile   string
	RunID        string
	RwxDirectory string
	Paths        []string // specific paths to pull, empty = all changed files
	Json         bool
}

type PullSandboxResult struct {
	PulledFiles []string
}

// Service methods

func (s Service) GetSandboxInitTemplate(cfg GetSandboxInitTemplateConfig) (*GetSandboxInitTemplateResult, error) {
	result, err := s.APIClient.GetSandboxInitTemplate()
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch sandbox init template")
	}

	return &GetSandboxInitTemplateResult{
		Template: result.Template,
	}, nil
}

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

	// Construct a descriptive title for the sandbox run
	title := SandboxTitle(cwd, branch, cfg.ConfigFile)

	runResult, err := s.InitiateRun(InitiateRunConfig{
		MintFilePath: cfg.ConfigFile,
		RwxDirectory: cfg.RwxDirectory,
		Json:         cfg.Json,
		Title:        title,
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

	// Execute command
	command := strings.Join(cfg.Command, " ")
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
	// Check once before showing spinner - sandbox may already be ready
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

	// Sandbox not ready yet - start spinner and poll
	var stopSpinner func()
	if !jsonMode {
		stopSpinner = Spin("Waiting for sandbox to be ready...", s.StdoutIsTTY, s.Stdout)
		defer stopSpinner()
	}

	for {
		// Use backoff from server, or default to 2 seconds
		backoffMs := 2000
		if connInfo.Polling.BackoffMs != nil {
			backoffMs = *connInfo.Polling.BackoffMs
		}
		time.Sleep(time.Duration(backoffMs) * time.Millisecond)

		connInfo, err = s.APIClient.GetSandboxConnectionInfo(runID)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get sandbox connection info")
		}

		if connInfo.Sandboxable {
			return &connInfo, nil
		}

		if connInfo.Polling.Completed {
			return nil, fmt.Errorf("Sandbox run '%s' completed before becoming ready", runID)
		}
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

	// Use ConnectWithKey to store the private key for use with external tools
	if err = s.SSHClient.ConnectWithKey(connInfo.Address, sshConfig, connInfo.PrivateUserKey); err != nil {
		return errors.Wrap(err, "unable to establish SSH connection to remote host")
	}

	return nil
}

// SandboxTitle constructs a descriptive title for sandbox runs using the
// project directory name, branch, and config file (if non-default).
func SandboxTitle(cwd, branch, configFile string) string {
	project := filepath.Base(cwd)
	if branch == "" {
		branch = "detached"
	}

	title := fmt.Sprintf("Sandbox: %s (%s)", project, branch)

	// Include config file if it's not the default
	if configFile != "" && configFile != ".rwx/sandbox.yml" {
		title = fmt.Sprintf("%s [%s]", title, configFile)
	}

	return title
}

func (s Service) syncChangesToSandbox(jsonMode bool) error {
	// Check for LFS files and warn (we use GitClient just for LFS detection)
	_, lfsFiles, _ := s.GitClient.GeneratePatch(nil)

	if lfsFiles != nil && lfsFiles.Count > 0 {
		if !jsonMode {
			fmt.Fprintf(s.Stderr, "Warning: %d LFS file(s) changed locally and cannot be synced.\n", lfsFiles.Count)
		}
	}

	// Get list of tracked files
	trackedFiles, err := s.getGitFiles(false)
	if err != nil {
		return errors.Wrap(err, "failed to get tracked files")
	}

	// Get list of untracked files (respecting .gitignore)
	untrackedFiles, err := s.getGitFiles(true)
	if err != nil {
		return errors.Wrap(err, "failed to get untracked files")
	}

	// Combine all files
	allFiles := append(trackedFiles, untrackedFiles...)

	// Skip sync if no files
	if len(allFiles) == 0 {
		return nil
	}

	// Sync files via tar
	return s.syncFilesViaTar(allFiles)
}

// getGitFiles returns files from git ls-files
// If untracked is true, returns untracked files (--others --exclude-standard)
// If untracked is false, returns tracked files
func (s Service) getGitFiles(untracked bool) ([]string, error) {
	var args []string
	if untracked {
		args = []string{"ls-files", "--others", "--exclude-standard"}
	} else {
		args = []string{"ls-files"}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "git ls-files failed")
	}

	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// syncFilesViaTar creates a tar archive of the specified files and extracts it on the sandbox
func (s Service) syncFilesViaTar(files []string) error {
	if len(files) == 0 {
		return nil
	}

	// Create tar command with file list via stdin (using -T -)
	// This avoids command line length limits with many files
	tarCmd := exec.Command("tar", "-cf", "-", "-T", "-")
	tarCmd.Stdin = strings.NewReader(strings.Join(files, "\n"))

	tarOutput, err := tarCmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create tar stdout pipe")
	}

	if err := tarCmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start tar command")
	}

	// Extract tar on sandbox in current working directory
	exitCode, err := s.SSHClient.ExecuteCommandWithStdin("tar -xf - 2>/dev/null", tarOutput)
	if err != nil {
		_ = tarCmd.Wait()
		return errors.Wrap(err, "failed to extract tar on sandbox")
	}

	if err := tarCmd.Wait(); err != nil {
		return errors.Wrap(err, "tar command failed")
	}

	if exitCode == 127 {
		return fmt.Errorf("tar is not installed in the sandbox")
	}
	if exitCode != 0 {
		return fmt.Errorf("tar extraction failed with exit code %d", exitCode)
	}

	return nil
}

func (s Service) PullSandbox(cfg PullSandboxConfig) (*PullSandboxResult, error) {
	defer s.outputLatestVersionMessage()

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current directory")
	}
	branch := GetCurrentGitBranch(cwd)

	var runID string
	var configFile string

	// Sandbox selection priority (same as ExecSandbox):
	// 1. --id flag
	// 2. Find by CWD + git branch in storage

	if cfg.RunID != "" {
		runID = cfg.RunID
		configFile = cfg.ConfigFile
	} else {
		storage, err := LoadSandboxStorage()
		if err != nil {
			return nil, errors.Wrap(err, "unable to load sandbox sessions")
		}

		if cfg.ConfigFile != "" {
			session, found := storage.GetSession(cwd, branch, cfg.ConfigFile)
			if !found {
				return nil, fmt.Errorf("No sandbox found for config file '%s'.\nUse 'rwx sandbox list' to see available sandboxes.", cfg.ConfigFile)
			}
			runID = session.RunID
			configFile = session.ConfigFile
		} else {
			sessions := storage.GetSessionsForCwdBranch(cwd, branch)
			var activeSessions []SandboxSession
			for _, sess := range sessions {
				connInfo, err := s.APIClient.GetSandboxConnectionInfo(sess.RunID)
				if err == nil && !connInfo.Polling.Completed {
					activeSessions = append(activeSessions, sess)
				}
			}

			if len(activeSessions) == 0 {
				return nil, fmt.Errorf("No active sandbox found for %s:%s.\nUse 'rwx sandbox list' to see available sandboxes.", cwd, branch)
			}
			if len(activeSessions) > 1 {
				return nil, fmt.Errorf("Multiple active sandboxes found for %s:%s.\nSpecify a config file to select one, or use --id to specify a run ID.", cwd, branch)
			}
			runID = activeSessions[0].RunID
			configFile = activeSessions[0].ConfigFile
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

	// Determine which files to pull
	var filesToPull []string
	if len(cfg.Paths) > 0 {
		filesToPull = cfg.Paths
	} else {
		// Get list of modified/new files on sandbox compared to git HEAD
		exitCode, output, err := s.SSHClient.ExecuteCommandWithOutput("{ git diff --name-only HEAD 2>/dev/null; git ls-files --others --exclude-standard 2>/dev/null; } | sort -u")
		if err != nil {
			return nil, errors.Wrap(err, "failed to get changed files from sandbox")
		}
		if exitCode != 0 {
			return nil, fmt.Errorf("failed to get changed files from sandbox (exit code %d)", exitCode)
		}

		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				filesToPull = append(filesToPull, line)
			}
		}
	}

	if len(filesToPull) == 0 {
		if !cfg.Json {
			fmt.Fprintln(s.Stdout, "No files to pull from sandbox.")
		}
		return &PullSandboxResult{PulledFiles: []string{}}, nil
	}

	// Pull files via tar
	pulledFiles, err := s.pullFilesViaTar(filesToPull)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pull files from sandbox")
	}

	if !cfg.Json {
		fmt.Fprintf(s.Stdout, "Pulled %d file(s) from sandbox:\n", len(pulledFiles))
		for _, f := range pulledFiles {
			fmt.Fprintf(s.Stdout, "  %s\n", f)
		}
	}

	return &PullSandboxResult{PulledFiles: pulledFiles}, nil
}

// pullFilesViaTar creates a tar archive on the sandbox and extracts it locally
func (s Service) pullFilesViaTar(files []string) ([]string, error) {
	if len(files) == 0 {
		return []string{}, nil
	}

	// Create tar on sandbox and stream to local extraction
	// Use -T - to read file list from stdin
	fileList := strings.Join(files, "\n")
	tarCommand := fmt.Sprintf("echo '%s' | tar -cf - -T - 2>/dev/null", fileList)

	// Create local tar extraction command
	tarExtract := exec.Command("tar", "-xf", "-")

	stdinPipe, err := tarExtract.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tar stdin pipe")
	}

	if err := tarExtract.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start local tar extraction")
	}

	// Execute remote tar and pipe output to local stdin
	exitCode, err := s.SSHClient.ExecuteCommandWithStdinAndStdout(tarCommand, nil, stdinPipe)
	stdinPipe.Close()

	if err != nil {
		_ = tarExtract.Wait()
		return nil, errors.Wrap(err, "failed to create tar on sandbox")
	}

	if err := tarExtract.Wait(); err != nil {
		return nil, errors.Wrap(err, "local tar extraction failed")
	}

	if exitCode == 127 {
		return nil, fmt.Errorf("tar is not installed in the sandbox")
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("tar creation on sandbox failed with exit code %d", exitCode)
	}

	return files, nil
}
