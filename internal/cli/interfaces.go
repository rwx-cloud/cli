package cli

import (
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/git"
	"github.com/rwx-cloud/cli/internal/ssh"

	gossh "golang.org/x/crypto/ssh"
)

type APIClient interface {
	GetDebugConnectionInfo(debugKey string) (api.DebugConnectionInfo, error)
	GetDispatch(api.GetDispatchConfig) (*api.GetDispatchResult, error)
	InitiateRun(api.InitiateRunConfig) (*api.InitiateRunResult, error)
	InitiateDispatch(api.InitiateDispatchConfig) (*api.InitiateDispatchResult, error)
	ObtainAuthCode(api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error)
	AcquireToken(tokenUrl string) (*api.AcquireTokenResult, error)
	Lint(api.LintConfig) (*api.LintResult, error)
	Whoami() (*api.WhoamiResult, error)
	SetSecretsInVault(api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error)
	GetPackageVersions() (*api.PackageVersionsResult, error)
	ResolveBaseLayer(api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error)
	StartImagePush(cfg api.StartImagePushConfig) (api.StartImagePushResult, error)
	ImagePushStatus(pushID string) (api.ImagePushStatusResult, error)
	TaskStatus(api.TaskStatusConfig) (api.TaskStatusResult, error)
	GetLogArchiveRequest(taskId string) (api.LogArchiveRequestResult, error)
	DownloadLogs(api.LogArchiveRequestResult) ([]byte, error)
}

var _ APIClient = api.Client{}

type SSHClient interface {
	Close() error
	Connect(addr string, cfg gossh.ClientConfig) error
	InteractiveSession() error
}

var _ SSHClient = (*ssh.Client)(nil)

type GitClient interface {
	GetBranch() string
	GetCommit() string
	GetOriginUrl() string
	GeneratePatchFile(destDir string) git.PatchFile
}
