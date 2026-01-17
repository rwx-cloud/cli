package cli

import (
	"io"
	"time"

	"github.com/distribution/reference"
	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/docker"
	"github.com/rwx-cloud/cli/internal/errors"
)

type Config struct {
	APIClient   APIClient
	SSHClient   SSHClient
	GitClient   GitClient
	DockerCLI   docker.Client
	Stdout      io.Writer
	StdoutIsTTY bool
	Stderr      io.Writer
	StderrIsTTY bool
}

func (c Config) Validate() error {
	if c.APIClient == nil {
		return errors.New("missing RWX client")
	}

	if c.SSHClient == nil {
		return errors.New("missing SSH client constructor")
	}

	if c.GitClient == nil {
		return errors.New("missing Git client constructor")
	}

	if c.DockerCLI == nil {
		return errors.New("missing Docker client")
	}

	if c.Stdout == nil {
		return errors.New("missing Stdout")
	}

	if c.Stderr == nil {
		return errors.New("missing Stderr")
	}

	return nil
}

type DebugTaskConfig struct {
	DebugKey string
}

func (c DebugTaskConfig) Validate() error {
	if c.DebugKey == "" {
		return errors.New("you must specify a run ID, a task ID, or an RWX Cloud URL")
	}

	return nil
}

type InitiateRunConfig struct {
	InitParameters map[string]string
	Json           bool
	RwxDirectory   string
	MintFilePath   string
	NoCache        bool
	TargetedTasks  []string
	Title          string
	GitBranch      string
	GitSha         string
}

func (c InitiateRunConfig) Validate() error {
	if c.MintFilePath == "" {
		return errors.New("the path to a run definition must be provided using the --file flag.")
	}

	return nil
}

type InitiateDispatchConfig struct {
	DispatchKey string
	Params      map[string]string
	Ref         string
	Json        bool
	Title       string
}

func (c InitiateDispatchConfig) Validate() error {
	if c.DispatchKey == "" {
		return errors.New("a dispatch key must be provided")
	}

	return nil
}

type GetDispatchConfig struct {
	DispatchId string
}

type GetDispatchRun struct {
	RunId  string
	RunUrl string
}

type LintOutputFormat int

const (
	LintOutputNone LintOutputFormat = iota
	LintOutputOneLine
	LintOutputMultiLine
	LintOutputJSON
)

type LintConfig struct {
	RwxDirectory string
	OutputFormat LintOutputFormat
}

func (c LintConfig) Validate() error {
	return nil
}

func NewLintConfig(rwxDir string, formatString string) (LintConfig, error) {
	var format LintOutputFormat

	switch formatString {
	case "none":
		format = LintOutputNone
	case "oneline":
		format = LintOutputOneLine
	case "multiline":
		format = LintOutputMultiLine
	case "json":
		format = LintOutputJSON
	default:
		return LintConfig{}, errors.New("unknown output format, expected one of: none, oneline, multiline, json")
	}

	return LintConfig{
		RwxDirectory: rwxDir,
		OutputFormat: format,
	}, nil
}

type LoginConfig struct {
	DeviceName         string
	AccessTokenBackend accesstoken.Backend
	OpenUrl            func(url string) error
	PollInterval       time.Duration
}

func (c LoginConfig) Validate() error {
	if c.DeviceName == "" {
		return errors.New("the device name must be provided")
	}

	return nil
}

type WhoamiConfig struct {
	Json bool
}

func (c WhoamiConfig) Validate() error {
	return nil
}

type SetSecretsInVaultConfig struct {
	Secrets []string
	Vault   string
	File    string
	Json    bool
}

func (c SetSecretsInVaultConfig) Validate() error {
	if c.Vault == "" {
		return errors.New("the vault name must be provided")
	}

	if len(c.Secrets) == 0 && c.File == "" {
		return errors.New("the secrets to set must be provided")
	}

	return nil
}

type UpdatePackagesConfig struct {
	RwxDirectory             string
	Files                    []string
	ReplacementVersionPicker func(versions api.PackageVersionsResult, rwxPackage string, major string) (string, error)
	Json                     bool
}

func (c UpdatePackagesConfig) Validate() error {
	if c.ReplacementVersionPicker == nil {
		return errors.New("a replacement version picker must be provided")
	}

	return nil
}

type InsertBaseConfig struct {
	RwxDirectory string
	Files        []string
	Json         bool
}

func (c InsertBaseConfig) Validate() error {
	return nil
}

type BaseSpec struct {
	Image  string `yaml:"image"`
	Config string `yaml:"config"`
	Arch   string `yaml:"arch"`
}

type BaseLayerRunFile struct {
	ResolvedBase BaseSpec
	OriginalPath string
	Error        error
}

type InsertDefaultBaseResult struct {
	ErroredRunFiles []BaseLayerRunFile
	UpdatedRunFiles []BaseLayerRunFile
}

func (r InsertDefaultBaseResult) HasChanges() bool {
	return len(r.ErroredRunFiles) > 0 || len(r.UpdatedRunFiles) > 0
}

type ResolvePackagesConfig struct {
	RwxDirectory        string
	Files               []string
	LatestVersionPicker func(versions api.PackageVersionsResult, rwxPackage string, _ string) (string, error)
	Json                bool
}

func (c ResolvePackagesConfig) PickLatestVersion(versions api.PackageVersionsResult, rwxPackage string) (string, error) {
	return c.LatestVersionPicker(versions, rwxPackage, "")
}

func (c ResolvePackagesConfig) Validate() error {
	if c.LatestVersionPicker == nil {
		return errors.New("a latest version picker must be provided")
	}

	return nil
}

type ResolvePackagesResult struct {
	ResolvedPackages map[string]string
}

func (r ResolvePackagesResult) HasChanges() bool {
	return len(r.ResolvedPackages) > 0
}

type ImagePushConfig struct {
	TaskID       string
	References   []reference.Named
	JSON         bool
	Wait         bool
	OpenURL      func(url string) error
	PollInterval time.Duration
}

func NewImagePushConfig(taskID string, references []string, json bool, wait bool, openURL func(url string) error) (ImagePushConfig, error) {
	if taskID == "" {
		return ImagePushConfig{}, errors.New("a task ID must be provided")
	}

	if len(references) == 0 {
		return ImagePushConfig{}, errors.New("at least one OCI reference must be provided")
	}

	parsedReferences := make([]reference.Named, 0, len(references))
	for _, refStr := range references {
		ref, err := reference.ParseNormalizedNamed(refStr)
		if err != nil {
			return ImagePushConfig{}, errors.Wrapf(err, "invalid OCI reference: %s", refStr)
		}
		parsedReferences = append(parsedReferences, ref)
	}

	return ImagePushConfig{
		TaskID:       taskID,
		References:   parsedReferences,
		JSON:         json,
		Wait:         wait,
		OpenURL:      openURL,
		PollInterval: 1 * time.Second,
	}, nil
}

type ImageBuildConfig struct {
	InitParameters   map[string]string
	RwxDirectory     string
	MintFilePath     string
	NoCache          bool
	NoPull           bool
	TargetTaskKey    string
	Tags             []string
	PushToReferences []string
	Timeout          time.Duration
	OpenURL          func(string) error
}

func (c ImageBuildConfig) Validate() error {
	if c.MintFilePath == "" {
		return errors.New("the path to a run definition must be provided")
	}
	if c.TargetTaskKey == "" {
		return errors.New("a target task key must be provided")
	}
	return nil
}

type ImagePullConfig struct {
	TaskID     string
	Tags       []string
	Timeout    time.Duration
	OutputJSON bool
}

func (c ImagePullConfig) Validate() error {
	if c.TaskID == "" {
		return errors.New("task ID must be provided")
	}
	return nil
}

type DownloadLogsConfig struct {
	TaskID      string
	OutputDir   string
	OutputFile  string
	Json        bool
	AutoExtract bool
	Open        bool
}

func (c DownloadLogsConfig) Validate() error {
	if c.TaskID == "" {
		return errors.New("task ID must be provided")
	}
	if c.OutputDir != "" && c.OutputFile != "" {
		return errors.New("output-dir and output-file cannot be used together")
	}
	return nil
}

type DownloadArtifactConfig struct {
	TaskID                 string
	ArtifactKey            string
	OutputDir              string
	OutputFile             string
	OutputDirExplicitlySet bool
	Json                   bool
	AutoExtract            bool
	Open                   bool
}

func (c DownloadArtifactConfig) Validate() error {
	if c.TaskID == "" {
		return errors.New("task ID must be provided")
	}
	if c.ArtifactKey == "" {
		return errors.New("artifact key must be provided")
	}
	if c.OutputDir != "" && c.OutputFile != "" {
		return errors.New("output-dir and output-file cannot be used together")
	}
	return nil
}

type GetRunStatusConfig struct {
	RunID string
	Wait  bool
	Json  bool
}

type GetRunStatusResult struct {
	RunID        string `json:"run_id"`
	ResultStatus string `json:"result_status"`
	Completed    bool   `json:"completed"`
}

type GetRunPromptResult struct {
	Prompt string `json:"prompt"`
}

type UpdatePackagesResult struct {
	UpdatedPackages map[string]string `json:"updated_packages"`
}

type DownloadArtifactResult struct {
	OutputFiles []string `json:"outputFiles"`
}

type DownloadLogsResult struct {
	OutputFiles []string `json:"outputFiles"`
}
