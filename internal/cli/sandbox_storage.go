package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/rwx-cloud/rwx/internal/errors"
	"github.com/rwx-cloud/rwx/internal/git"
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
	RunID       string     `json:"runId"`
	ConfigFile  string     `json:"configFile"`
	ScopedToken string     `json:"scopedToken,omitempty"`
	RunURL      string     `json:"runUrl,omitempty"`
	ConfigHash  string     `json:"configHash,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	LastExecAt  *time.Time `json:"lastExecAt,omitempty"`
	ExecCount   int        `json:"execCount,omitempty"`
}

// HashConfigFile returns a hex-encoded SHA-256 hash of the file at the given path.
// Returns an empty string if the file cannot be read.
func HashConfigFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type SandboxStorage struct {
	Sandboxes map[string]SandboxSession `json:"sandboxes"`
}

func sandboxStoragePath() (string, error) {
	rwxDir, err := findRwxDirectoryPath("")
	if err != nil {
		return "", err
	}

	if rwxDir == "" {
		rwxDir, err = createRwxDirectory()
		if err != nil {
			return "", err
		}
	}

	return filepath.Join(rwxDir, "sandboxes", "sandboxes.json"), nil
}

func createRwxDirectory() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "unable to determine the working directory")
	}

	// Prefer the git repository root so the .rwx directory sits alongside .git
	client := &git.Client{Binary: "git", Dir: cwd}
	if topLevel := client.GetTopLevel(); topLevel != "" {
		cwd = topLevel
	}

	rwxDir := filepath.Join(cwd, ".rwx")
	if err := os.MkdirAll(rwxDir, 0o755); err != nil {
		return "", errors.Wrapf(err, "unable to create %q", rwxDir)
	}

	return rwxDir, nil
}

type SandboxStorageLock struct {
	flock *flock.Flock
}

// LockSandboxStorage acquires an exclusive file lock to serialize concurrent
// CLI processes that resolve and create sandbox sessions.
func LockSandboxStorage() (*SandboxStorageLock, error) {
	storagePath, err := sandboxStoragePath()
	if err != nil {
		return nil, err
	}

	lockPath := storagePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), os.ModePerm); err != nil {
		return nil, err
	}

	lock := &SandboxStorageLock{flock: flock.New(lockPath)}
	if err := lock.flock.Lock(); err != nil {
		return nil, err
	}

	return lock, nil
}

// TryLockSandboxStorage attempts to acquire the lock without blocking.
// Returns an error if the lock is already held by another process.
func TryLockSandboxStorage() (*SandboxStorageLock, error) {
	storagePath, err := sandboxStoragePath()
	if err != nil {
		return nil, err
	}

	lockPath := storagePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), os.ModePerm); err != nil {
		return nil, err
	}

	lock := &SandboxStorageLock{flock: flock.New(lockPath)}
	locked, err := lock.flock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("sandbox storage is locked")
	}

	return lock, nil
}

func UnlockSandboxStorage(lock *SandboxStorageLock) {
	if lock == nil {
		return
	}
	if lock.flock != nil {
		_ = lock.flock.Unlock()
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

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "unable to create directory for %q", path)
	}

	ensureSandboxesDirGitignore(dir)

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

func ensureSandboxesDirGitignore(dir string) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		_ = os.WriteFile(gitignorePath, []byte("*\n"), 0644)
	}
}

func SessionKey(cwd, branch, configFile string) string {
	if branch == "" {
		branch = "detached"
	}
	if configFile != "" && !filepath.IsAbs(configFile) {
		panic(fmt.Sprintf("SessionKey called with relative configFile: %q", configFile))
	}
	return fmt.Sprintf("%s:%s:%s", cwd, branch, configFile)
}

// AbsConfigFile coerces a config file path to be absolute. If the path is
// already absolute it is returned as-is; otherwise it is resolved relative to
// the current working directory.
func AbsConfigFile(configFile string) string {
	if configFile == "" || filepath.IsAbs(configFile) {
		return configFile
	}
	abs, err := filepath.Abs(configFile)
	if err != nil {
		return configFile
	}
	return abs
}

// IsDetachedBranch returns true if the branch string represents a detached HEAD.
func IsDetachedBranch(branch string) bool {
	return branch == "detached" || strings.HasPrefix(branch, "detached@")
}

// DetachedShortSHA extracts the short SHA from a "detached@<sha>" branch string.
// Returns empty string if the branch is not in detached format or has no SHA.
func DetachedShortSHA(branch string) string {
	if strings.HasPrefix(branch, "detached@") {
		return branch[len("detached@"):]
	}
	return ""
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
	if branch == "" {
		branch = "detached"
	}
	prefix := cwd + ":" + branch + ":"
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
		// Detached HEAD — use the short SHA so session keys are unique per commit
		shortSHA := client.GetShortHead()
		if shortSHA == "" {
			return "detached"
		}
		return "detached@" + shortSHA
	}
	return branch
}

// AncestryChecker abstracts the git ancestor check so callers can inject a
// real git.Client or a test mock.
type AncestryChecker interface {
	IsAncestor(candidateSHA, headRef string) bool
}

// GetSessionByAncestry falls back to ancestry-based lookup when the current
// branch is detached and an exact key match was not found. If a stored session's
// detached SHA is an ancestor of HEAD, the session is returned and re-keyed to
// the current branch so subsequent lookups hit the fast path.
// The caller must call Save() to persist the re-keyed session.
func (s *SandboxStorage) GetSessionByAncestry(cwd, branch, configFile string, checker AncestryChecker) (*SandboxSession, bool) {
	if !IsDetachedBranch(branch) || DetachedShortSHA(branch) == "" {
		return nil, false
	}

	for key, session := range s.Sandboxes {
		storedCwd, storedBranch, storedConfig := ParseSessionKey(key)
		if storedCwd != cwd || storedConfig != configFile {
			continue
		}
		if !IsDetachedBranch(storedBranch) {
			continue
		}
		storedSHA := DetachedShortSHA(storedBranch)
		if storedSHA == "" {
			continue
		}
		if checker.IsAncestor(storedSHA, "HEAD") {
			delete(s.Sandboxes, key)
			newKey := SessionKey(cwd, branch, configFile)
			s.Sandboxes[newKey] = session
			return &session, true
		}
	}
	return nil, false
}

// GetSessionsForCwdBranchByAncestry returns sessions where the stored detached
// SHA is an ancestor of HEAD. Matching sessions are re-keyed to the current branch.
// The caller must call Save() to persist key changes.
func (s *SandboxStorage) GetSessionsForCwdBranchByAncestry(cwd, branch string, checker AncestryChecker) []SandboxSession {
	if !IsDetachedBranch(branch) || DetachedShortSHA(branch) == "" {
		return nil
	}

	type rekey struct {
		oldKey  string
		config  string
		session SandboxSession
	}
	var toRekey []rekey

	for key, session := range s.Sandboxes {
		storedCwd, storedBranch, storedConfig := ParseSessionKey(key)
		if storedCwd != cwd {
			continue
		}
		if !IsDetachedBranch(storedBranch) {
			continue
		}
		storedSHA := DetachedShortSHA(storedBranch)
		if storedSHA == "" {
			continue
		}
		if checker.IsAncestor(storedSHA, "HEAD") {
			toRekey = append(toRekey, rekey{oldKey: key, config: storedConfig, session: session})
		}
	}

	var sessions []SandboxSession
	for _, r := range toRekey {
		delete(s.Sandboxes, r.oldKey)
		newKey := SessionKey(cwd, branch, r.config)
		s.Sandboxes[newKey] = r.session
		sessions = append(sessions, r.session)
	}

	return sessions
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
