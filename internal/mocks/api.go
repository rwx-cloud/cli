package mocks

import (
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
)

type API struct {
	MockInitiateRun                func(api.InitiateRunConfig) (*api.InitiateRunResult, error)
	MockGetDebugConnectionInfo     func(runID string) (api.DebugConnectionInfo, error)
	MockGetSandboxConnectionInfo   func(runID, scopedToken string) (api.SandboxConnectionInfo, error)
	MockCreateSandboxToken         func(api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error)
	MockObtainAuthCode             func(api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error)
	MockAcquireToken               func(tokenUrl string) (*api.AcquireTokenResult, error)
	MockWhoami                     func() (*api.WhoamiResult, error)
	MockSetSecretsInVault          func(api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error)
	MockGetPackageVersions         func() (*api.PackageVersionsResult, error)
	MockGetPackageDocumentation    func(string) (*api.PackageDocumentationResult, error)
	MockInitiateDispatch           func(api.InitiateDispatchConfig) (*api.InitiateDispatchResult, error)
	MockGetDispatch                func(api.GetDispatchConfig) (*api.GetDispatchResult, error)
	MockGetDefaultBase             func() (api.DefaultBaseResult, error)
	MockMcpGetRunTestFailures      func(api.McpGetRunTestFailuresRequest) (*api.McpTextResult, error)
	MockStartImagePush             func(api.StartImagePushConfig) (api.StartImagePushResult, error)
	MockImagePushStatus            func(string) (api.ImagePushStatusResult, error)
	MockTaskKeyStatus              func(api.TaskKeyStatusConfig) (api.TaskStatusResult, error)
	MockTaskIDStatus               func(api.TaskIDStatusConfig) (api.TaskStatusResult, error)
	MockRunStatus                  func(api.RunStatusConfig) (api.RunStatusResult, error)
	MockGetLogDownloadRequest      func(string) (api.LogDownloadRequestResult, error)
	MockDownloadLogs               func(api.LogDownloadRequestResult) ([]byte, error)
	MockGetArtifactDownloadRequest func(string, string) (api.ArtifactDownloadRequestResult, error)
	MockDownloadArtifact           func(api.ArtifactDownloadRequestResult) ([]byte, error)
	MockGetRunPrompt               func(string) (string, error)
	MockGetSandboxInitTemplate     func() (api.SandboxInitTemplateResult, error)
}

func (c *API) InitiateRun(cfg api.InitiateRunConfig) (*api.InitiateRunResult, error) {
	if c.MockInitiateRun != nil {
		return c.MockInitiateRun(cfg)
	}

	return nil, errors.New("MockInitiateRun was not configured")
}

func (c *API) GetDebugConnectionInfo(runID string) (api.DebugConnectionInfo, error) {
	if c.MockGetDebugConnectionInfo != nil {
		return c.MockGetDebugConnectionInfo(runID)
	}

	return api.DebugConnectionInfo{}, errors.New("MockGetDebugConnectionInfo was not configured")
}

func (c *API) GetSandboxConnectionInfo(runID, scopedToken string) (api.SandboxConnectionInfo, error) {
	if c.MockGetSandboxConnectionInfo != nil {
		return c.MockGetSandboxConnectionInfo(runID, scopedToken)
	}

	return api.SandboxConnectionInfo{}, errors.New("MockGetSandboxConnectionInfo was not configured")
}

func (c *API) CreateSandboxToken(cfg api.CreateSandboxTokenConfig) (*api.CreateSandboxTokenResult, error) {
	if c.MockCreateSandboxToken != nil {
		return c.MockCreateSandboxToken(cfg)
	}

	return nil, errors.New("MockCreateSandboxToken was not configured")
}

func (c *API) ObtainAuthCode(cfg api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
	if c.MockObtainAuthCode != nil {
		return c.MockObtainAuthCode(cfg)
	}

	return nil, errors.New("MockObtainAuthCode was not configured")
}

func (c *API) AcquireToken(tokenUrl string) (*api.AcquireTokenResult, error) {
	if c.MockAcquireToken != nil {
		return c.MockAcquireToken(tokenUrl)
	}

	return nil, errors.New("MockAcquireToken was not configured")
}

func (c *API) Whoami() (*api.WhoamiResult, error) {
	if c.MockWhoami != nil {
		return c.MockWhoami()
	}

	return nil, errors.New("MockWhoami was not configured")
}

func (c *API) SetSecretsInVault(cfg api.SetSecretsInVaultConfig) (*api.SetSecretsInVaultResult, error) {
	if c.MockSetSecretsInVault != nil {
		return c.MockSetSecretsInVault(cfg)
	}

	return nil, errors.New("MockSetSecretsInVault was not configured")
}

func (c *API) GetPackageVersions() (*api.PackageVersionsResult, error) {
	if c.MockGetPackageVersions != nil {
		return c.MockGetPackageVersions()
	}

	return nil, errors.New("MockGetPackageVersions was not configured")
}

func (c *API) GetPackageDocumentation(packageName string) (*api.PackageDocumentationResult, error) {
	if c.MockGetPackageDocumentation != nil {
		return c.MockGetPackageDocumentation(packageName)
	}

	return nil, errors.New("MockGetPackageDocumentation was not configured")
}

func (c *API) InitiateDispatch(cfg api.InitiateDispatchConfig) (*api.InitiateDispatchResult, error) {
	if c.MockInitiateDispatch != nil {
		return c.MockInitiateDispatch(cfg)
	}

	return nil, errors.New("MockInitiateDispatch was not configured")
}

func (c *API) GetDispatch(cfg api.GetDispatchConfig) (*api.GetDispatchResult, error) {
	if c.MockGetDispatch != nil {
		return c.MockGetDispatch(cfg)
	}

	return nil, errors.New("MockGetDispatch was not configured")
}

func (c *API) GetDefaultBase() (api.DefaultBaseResult, error) {
	if c.MockGetDefaultBase != nil {
		return c.MockGetDefaultBase()
	}

	return api.DefaultBaseResult{}, errors.New("MockGetDefaultBase was not configured")
}

func (c *API) McpGetRunTestFailures(cfg api.McpGetRunTestFailuresRequest) (*api.McpTextResult, error) {
	if c.MockMcpGetRunTestFailures != nil {
		return c.MockMcpGetRunTestFailures(cfg)
	}

	return nil, errors.New("MockMcpGetRunTestFailures was not configured")
}

func (c *API) StartImagePush(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
	if c.MockStartImagePush != nil {
		return c.MockStartImagePush(cfg)
	}

	return api.StartImagePushResult{}, errors.New("MockStartImagePush was not configured")
}

func (c *API) ImagePushStatus(pushID string) (api.ImagePushStatusResult, error) {
	if c.MockImagePushStatus != nil {
		return c.MockImagePushStatus(pushID)
	}

	return api.ImagePushStatusResult{}, errors.New("MockImagePushStatus was not configured")
}

func (c *API) TaskKeyStatus(cfg api.TaskKeyStatusConfig) (api.TaskStatusResult, error) {
	if c.MockTaskKeyStatus != nil {
		return c.MockTaskKeyStatus(cfg)
	}

	return api.TaskStatusResult{}, errors.New("MockTaskKeyStatus was not configured")
}

func (c *API) TaskIDStatus(cfg api.TaskIDStatusConfig) (api.TaskStatusResult, error) {
	if c.MockTaskIDStatus != nil {
		return c.MockTaskIDStatus(cfg)
	}

	return api.TaskStatusResult{}, errors.New("MockTaskIDStatus was not configured")
}

func (c *API) RunStatus(cfg api.RunStatusConfig) (api.RunStatusResult, error) {
	if c.MockRunStatus != nil {
		return c.MockRunStatus(cfg)
	}

	return api.RunStatusResult{}, errors.New("MockRunStatus was not configured")
}

func (c *API) GetLogDownloadRequest(taskId string) (api.LogDownloadRequestResult, error) {
	if c.MockGetLogDownloadRequest != nil {
		return c.MockGetLogDownloadRequest(taskId)
	}

	return api.LogDownloadRequestResult{}, errors.New("MockGetLogDownloadRequest was not configured")
}

func (c *API) DownloadLogs(request api.LogDownloadRequestResult, maxRetryDurationSeconds ...int) ([]byte, error) {
	if c.MockDownloadLogs != nil {
		return c.MockDownloadLogs(request)
	}

	return nil, errors.New("MockDownloadLogs was not configured")
}

func (c *API) GetArtifactDownloadRequest(taskId, artifactKey string) (api.ArtifactDownloadRequestResult, error) {
	if c.MockGetArtifactDownloadRequest != nil {
		return c.MockGetArtifactDownloadRequest(taskId, artifactKey)
	}

	return api.ArtifactDownloadRequestResult{}, errors.New("MockGetArtifactDownloadRequest was not configured")
}

func (c *API) DownloadArtifact(request api.ArtifactDownloadRequestResult) ([]byte, error) {
	if c.MockDownloadArtifact != nil {
		return c.MockDownloadArtifact(request)
	}

	return nil, errors.New("MockDownloadArtifact was not configured")
}

func (c *API) GetRunPrompt(runID string) (string, error) {
	if c.MockGetRunPrompt != nil {
		return c.MockGetRunPrompt(runID)
	}

	return "", errors.New("MockGetRunPrompt was not configured")
}

func (c *API) GetSandboxInitTemplate() (api.SandboxInitTemplateResult, error) {
	if c.MockGetSandboxInitTemplate != nil {
		return c.MockGetSandboxInitTemplate()
	}

	return api.SandboxInitTemplateResult{}, errors.New("MockGetSandboxInitTemplate was not configured")
}
