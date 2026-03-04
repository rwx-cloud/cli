package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/git"
)

type CliState struct {
	CWD        string `json:"cwd"`
	Branch     string `json:"branch"`
	ConfigFile string `json:"configFile"`
}

func EncodeCliState(cwd, branch, configFile string) string {
	state := CliState{CWD: cwd, Branch: branch, ConfigFile: configFile}
	data, _ := json.Marshal(state)
	return base64.StdEncoding.EncodeToString(data)
}

func DecodeCliState(encoded string) (*CliState, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode cli_state")
	}
	var state CliState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, errors.Wrap(err, "unable to parse cli_state")
	}
	return &state, nil
}

type SandboxSession struct {
	RunID       string `json:"runId"`
	ConfigFile  string `json:"configFile"`
	ScopedToken string `json:"scopedToken,omitempty"`
	RunURL      string `json:"runUrl,omitempty"`
}

type SandboxStorage struct {
	Sandboxes map[string]SandboxSession `json:"sandboxes"`
}

func sandboxStoragePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "unable to get home directory")
	}
	return filepath.Join(homeDir, ".config", "rwx", "sandboxes.json"), nil
}

// LockSandboxStorage acquires an exclusive file lock to serialize concurrent
// CLI processes that resolve and create sandbox sessions.
func LockSandboxStorage() (*flock.Flock, error) {
	path, err := sandboxStoragePath()
	if err != nil {
		return nil, err
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), os.ModePerm); err != nil {
		return nil, err
	}
	fl := flock.New(lockPath)
	if err := fl.Lock(); err != nil {
		return nil, err
	}
	return fl, nil
}

// UnlockSandboxStorage releases the file lock acquired by LockSandboxStorage.
func UnlockSandboxStorage(fl *flock.Flock) {
	if fl != nil {
		_ = fl.Unlock()
	}
}

func LoadSandboxStorage() (*SandboxStorage, error) {
	path, err := sandboxStoragePath()
	if err != nil {
		return nil, err
	}

	storage := &SandboxStorage{
		Sandboxes: make(map[string]SandboxSession),
	}

	fd, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return storage, nil
		}
		return nil, errors.Wrapf(err, "unable to open %q", path)
	}
	defer fd.Close()

	if err := json.NewDecoder(fd).Decode(storage); err != nil {
		return nil, errors.Wrapf(err, "unable to parse %q", path)
	}

	if storage.Sandboxes == nil {
		storage.Sandboxes = make(map[string]SandboxSession)
	}

	return storage, nil
}

func (s *SandboxStorage) Save() error {
	path, err := sandboxStoragePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return errors.Wrapf(err, "unable to create directory for %q", path)
	}

	fd, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "unable to create %q", path)
	}
	defer fd.Close()

	encoder := json.NewEncoder(fd)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s); err != nil {
		return errors.Wrapf(err, "unable to write %q", path)
	}

	return nil
}

func SessionKey(cwd, branch, configFile string) string {
	if branch == "" {
		branch = "detached"
	}
	return fmt.Sprintf("%s:%s:%s", cwd, branch, configFile)
}

func (s *SandboxStorage) GetSession(cwd, branch, configFile string) (*SandboxSession, bool) {
	key := SessionKey(cwd, branch, configFile)
	session, found := s.Sandboxes[key]
	if !found {
		return nil, false
	}
	return &session, true
}

// GetSessionsForCwdBranch returns all sessions matching cwd and branch (any config file)
func (s *SandboxStorage) GetSessionsForCwdBranch(cwd, branch string) []SandboxSession {
	prefix := cwd + ":" + branch + ":"
	if branch == "" {
		prefix = cwd + ":detached:"
	}
	var sessions []SandboxSession
	for key, session := range s.Sandboxes {
		if strings.HasPrefix(key, prefix) {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func (s *SandboxStorage) SetSession(cwd, branch, configFile string, session SandboxSession) {
	key := SessionKey(cwd, branch, configFile)
	s.Sandboxes[key] = session
}

func (s *SandboxStorage) DeleteSession(cwd, branch, configFile string) {
	key := SessionKey(cwd, branch, configFile)
	delete(s.Sandboxes, key)
}

func (s *SandboxStorage) DeleteSessionByRunID(runID string) bool {
	for key, session := range s.Sandboxes {
		if session.RunID == runID {
			delete(s.Sandboxes, key)
			return true
		}
	}
	return false
}

func (s *SandboxStorage) FindByRunID(runID string) (*SandboxSession, string, bool) {
	for key, session := range s.Sandboxes {
		if session.RunID == runID {
			return &session, key, true
		}
	}
	return nil, "", false
}

func (s *SandboxStorage) AllSessions() map[string]SandboxSession {
	return s.Sandboxes
}

func GetCurrentGitBranch(cwd string) string {
	client := &git.Client{Binary: "git", Dir: cwd}
	branch := client.GetBranch()
	if branch == "" {
		return "detached"
	}
	return branch
}

func ParseSessionKey(key string) (cwd, branch, configFile string) {
	// Key format: cwd:branch:configFile
	// Find last two colons
	lastColon := strings.LastIndex(key, ":")
	if lastColon == -1 {
		return key, "", ""
	}
	configFile = key[lastColon+1:]
	rest := key[:lastColon]

	secondLastColon := strings.LastIndex(rest, ":")
	if secondLastColon == -1 {
		return rest, "", configFile
	}
	return rest[:secondLastColon], rest[secondLastColon+1:], configFile
}
