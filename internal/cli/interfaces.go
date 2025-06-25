package cli

import (
	"github.com/rwx-cloud/cli/internal/api"

	"golang.org/x/crypto/ssh"
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
}

type SSHClient interface {
	Close() error
	Connect(addr string, cfg ssh.ClientConfig) error
	InteractiveSession() error
}
