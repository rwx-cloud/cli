package api

import (
	"bytes"
	"fmt"
	"io"

	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/messages"
)

type Config struct {
	Host               string
	AccessToken        string
	AccessTokenBackend accesstoken.Backend
}

func (c Config) Validate() error {
	if c.Host == "" {
		return errors.New("missing host")
	}

	return nil
}

type InitiateRunConfig struct {
	InitializationParameters []InitializationParameter `json:"initialization_parameters"`
	TaskDefinitions          []RwxDirectoryEntry       `json:"task_definitions"`
	RwxDirectory             []RwxDirectoryEntry       `json:"mint_directory"`
	TargetedTaskKeys         []string                  `json:"targeted_task_keys,omitempty"`
	Title                    string                    `json:"title,omitempty"`
	UseCache                 bool                      `json:"use_cache"`
	Git                      GitMetadata               `json:"git"`
	Patch                    PatchMetadata             `json:"patch"`
}

type InitializationParameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type GitMetadata struct {
	Branch    string `json:"branch"`
	Sha       string `json:"sha"`
	OriginUrl string `json:"origin_url"`
}

type PatchMetadata struct {
	Sent           bool     `json:"sent"`
	LFSFiles       []string `json:"lfs_files"`
	LFSCount       int      `json:"lfs_count"`
	UntrackedFiles []string `json:"untracked_files"`
	UntrackedCount int      `json:"untracked_count"`
}

type InitiateRunResult struct {
	RunId            string
	RunURL           string
	TargetedTaskKeys []string
	DefinitionPath   string
	Message          string
}

func (c InitiateRunConfig) Validate() error {
	if len(c.TaskDefinitions) == 0 {
		return errors.New("no task definitions")
	}

	return nil
}

type InitiateDispatchConfig struct {
	DispatchKey string            `json:"key"`
	Params      map[string]string `json:"params"`
	Title       string            `json:"title,omitempty"`
	Ref         string            `json:"ref,omitempty"`
}

type InitiateDispatchResult struct {
	DispatchId string
}

func (c InitiateDispatchConfig) Validate() error {
	if c.DispatchKey == "" {
		return errors.New("no dispatch key was provided")
	}

	return nil
}

type GetDispatchConfig struct {
	DispatchId string
}

type GetDispatchRun = struct {
	RunId  string `json:"run_id"`
	RunUrl string `json:"run_url"`
}

type GetDispatchResult struct {
	Status string
	Error  string
	Runs   []GetDispatchRun
}

type LintConfig struct {
	TaskDefinitions []TaskDefinition `json:"task_definitions"`
	TargetPaths     []string         `json:"target_paths"`
}

func (c LintConfig) Validate() error {
	if len(c.TaskDefinitions) == 0 {
		return errors.New("no task definitions")
	}

	if len(c.TargetPaths) == 0 {
		return errors.New("no target paths")
	}

	return nil
}

type LintProblem struct {
	Severity   string                `json:"severity"`
	Message    string                `json:"message"`
	FileName   string                `json:"file_name"`
	Line       NullInt               `json:"line"`
	Column     NullInt               `json:"column"`
	Advice     string                `json:"advice"`
	StackTrace []messages.StackEntry `json:"stack_trace,omitempty"`
	Frame      string                `json:"frame"`
}

func (lf LintProblem) FileLocation() string {
	fileName := lf.FileName
	line := lf.Line
	column := lf.Column

	if len(lf.StackTrace) > 0 {
		lastStackEntry := lf.StackTrace[len(lf.StackTrace)-1]
		fileName = lastStackEntry.FileName
		line = NullInt{
			Value:  lastStackEntry.Line,
			IsNull: false,
		}
		column = NullInt{
			Value:  lastStackEntry.Column,
			IsNull: false,
		}
	}

	if len(fileName) > 0 {
		var buf bytes.Buffer
		w := io.Writer(&buf)

		fmt.Fprint(w, fileName)

		if !line.IsNull {
			fmt.Fprintf(w, ":%d", line.Value)
		}
		if !column.IsNull {
			fmt.Fprintf(w, ":%d", column.Value)
		}

		return buf.String()
	}

	return ""
}

type LintResult struct {
	Problems []LintProblem `json:"problems"`
}

type ObtainAuthCodeConfig struct {
	Code ObtainAuthCodeCode `json:"code"`
}

type ObtainAuthCodeCode struct {
	DeviceName string `json:"device_name"`
}

type ObtainAuthCodeResult struct {
	AuthorizationUrl string `json:"authorization_url"`
	TokenUrl         string `json:"token_url"`
}

func (c ObtainAuthCodeConfig) Validate() error {
	if c.Code.DeviceName == "" {
		return errors.New("device name must be provided")
	}

	return nil
}

type AcquireTokenResult struct {
	State string `json:"state"` // consumed, expired, authorized, pending
	Token string `json:"token,omitempty"`
}

type WhoamiResult struct {
	OrganizationSlug string  `json:"organization_slug"`
	TokenKind        string  `json:"token_kind"` // organization_access_token, personal_access_token
	UserEmail        *string `json:"user_email,omitempty"`
}

type SetSecretsInVaultConfig struct {
	Secrets   []Secret `json:"secrets"`
	VaultName string   `json:"vault_name"`
}

type Secret struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

type SetSecretsInVaultResult struct {
	SetSecrets []string `json:"set_secrets"`
}

type PackageVersionsResult struct {
	Renames     map[string]string            `json:"renames"`
	LatestMajor map[string]string            `json:"latest_major"`
	LatestMinor map[string]map[string]string `json:"latest_minor"`
}

type DefaultBaseResult struct {
	Image  string `json:"image,omitempty"`
	Config string `json:"config,omitempty"`
	Arch   string `json:"arch,omitempty"`
}

type StartImagePushConfig struct {
	TaskID      string                          `json:"task_id"`
	Image       StartImagePushConfigImage       `json:"image"`
	Credentials StartImagePushConfigCredentials `json:"credentials"`
}

type StartImagePushConfigImage struct {
	Registry   string   `json:"registry"`
	Repository string   `json:"repository"`
	Tags       []string `json:"tags"`
}

type StartImagePushConfigCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type StartImagePushResult struct {
	PushID string `json:"push_id"`
	RunURL string `json:"run_url"`
}

type ImagePushStatusResult struct {
	Status string `json:"status"`
}

type TaskKeyStatusConfig struct {
	RunID   string
	TaskKey string
}

type TaskIDStatusConfig struct {
	TaskID string
}

const (
	TaskStatusSucceeded = "succeeded"
)

type PollingResult struct {
	Completed bool `json:"completed"`
	BackoffMs *int `json:"backoff_ms,omitempty"`
}

type TaskStatus struct {
	Result string `json:"result"`
}

type TaskStatusResult struct {
	Status  *TaskStatus   `json:"status,omitempty"`
	TaskID  string        `json:"task_id,omitempty"`
	Polling PollingResult `json:"polling"`
}

type LogDownloadRequestResult struct {
	URL      string  `json:"url"`
	Token    string  `json:"token"`
	Filename string  `json:"filename"`
	Contents *string `json:"contents,omitempty"`
}

type ArtifactDownloadRequestResult struct {
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	SizeInBytes int64  `json:"size_in_bytes"`
	Kind        string `json:"kind"`
	Key         string `json:"key"`
}

type RunStatusConfig struct {
	RunID string
}

type RunStatus struct {
	Result string `json:"result"`
}

type RunStatusResult struct {
	Status  *RunStatus    `json:"run_status,omitempty"`
	RunID   string        `json:"run_id,omitempty"`
	Polling PollingResult `json:"polling"`
}

type SandboxInitTemplateResult struct {
	Template string `json:"template"`
}
