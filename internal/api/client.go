package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rwx-cloud/cli/cmd/rwx/config"
	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/rwx-cloud/cli/internal/messages"
	"github.com/rwx-cloud/cli/internal/versions"
)

var ErrNotFound = errors.New("not found")

type roundTripFunc func(*http.Request) (*http.Response, error)

func (rtf roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rtf(r)
}

// Client is an API Client for Mint
type Client struct {
	http.RoundTripper
}

func NewClient(cfg Config) (Client, error) {
	if err := cfg.Validate(); err != nil {
		return Client{}, errors.Wrap(err, "validation failed")
	}

	roundTrip := func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme == "" {
			req.URL.Scheme = "https"
		}
		if req.URL.Host == "" {
			req.URL.Host = cfg.Host
		}
		req.Header.Set("User-Agent", fmt.Sprintf("rwx-cli/%s", config.Version))

		token, err := accesstoken.Get(cfg.AccessTokenBackend, cfg.AccessToken)
		if err != nil {
			return nil, errors.Wrap(err, "unable to retrieve access token")
		}
		if token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}

		return http.DefaultClient.Do(req)
	}

	roundTripper := versions.NewRoundTripper(roundTripFunc(roundTrip), cfg.VersionsBackend)
	return Client{roundTripper}, nil
}

func NewClientWithRoundTrip(rt func(*http.Request) (*http.Response, error)) Client {
	return Client{roundTripFunc(rt)}
}

func (c Client) GetDebugConnectionInfo(debugKey string) (DebugConnectionInfo, error) {
	connectionInfo := DebugConnectionInfo{}

	if debugKey == "" {
		return connectionInfo, errors.New("missing debugKey")
	}

	endpoint := fmt.Sprintf("/mint/api/debug_connection_info?debug_key=%s", url.QueryEscape(debugKey))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return connectionInfo, errors.Wrap(err, "unable to create new HTTP request")
	}

	resp, err := c.RoundTrip(req)
	if err != nil {
		return connectionInfo, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		break
	case 400:
		connectionError := DebugConnectionInfoError{}
		if err := json.NewDecoder(resp.Body).Decode(&connectionError); err == nil {
			return connectionInfo, errors.Wrap(errors.ErrBadRequest, connectionError.Error)
		}
		return connectionInfo, errors.ErrBadRequest
	case 404:
		return connectionInfo, errors.ErrNotFound
	case 410:
		connectionError := DebugConnectionInfoError{}
		if err := json.NewDecoder(resp.Body).Decode(&connectionError); err == nil {
			return connectionInfo, errors.Wrap(errors.ErrGone, connectionError.Error)
		}
		return connectionInfo, errors.ErrGone
	default:
		return connectionInfo, errors.New(fmt.Sprintf("Unable to call RWX API - %s", resp.Status))
	}

	if err := json.NewDecoder(resp.Body).Decode(&connectionInfo); err != nil {
		return connectionInfo, errors.Wrap(err, "unable to parse API response")
	}

	return connectionInfo, nil
}

func (c Client) GetSandboxConnectionInfo(runID, scopedToken string) (SandboxConnectionInfo, error) {
	connectionInfo := SandboxConnectionInfo{}

	if runID == "" {
		return connectionInfo, errors.New("missing runID")
	}

	endpoint := fmt.Sprintf("/mint/api/sandbox_connection_info?sandbox_key=%s", url.QueryEscape(runID))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return connectionInfo, errors.Wrap(err, "unable to create new HTTP request")
	}

	// If a scoped token is provided, set it directly on the request
	// This overrides the default token from the RoundTripper
	if scopedToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", scopedToken))
	}

	resp, err := c.RoundTrip(req)
	if err != nil {
		return connectionInfo, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		break
	case 400:
		connectionError := SandboxConnectionInfoError{}
		if err := json.NewDecoder(resp.Body).Decode(&connectionError); err == nil && connectionError.Error != "" {
			return connectionInfo, errors.New(connectionError.Error)
		}
		return connectionInfo, errors.ErrBadRequest
	case 404:
		connectionError := SandboxConnectionInfoError{}
		if err := json.NewDecoder(resp.Body).Decode(&connectionError); err == nil && connectionError.Error != "" {
			return connectionInfo, errors.New(connectionError.Error)
		}
		return connectionInfo, errors.ErrNotFound
	case 410:
		connectionError := SandboxConnectionInfoError{}
		if err := json.NewDecoder(resp.Body).Decode(&connectionError); err == nil && connectionError.Error != "" {
			return connectionInfo, errors.New(connectionError.Error)
		}
		return connectionInfo, errors.ErrGone
	default:
		return connectionInfo, errors.New(fmt.Sprintf("Unable to call RWX API - %s", resp.Status))
	}

	if err := json.NewDecoder(resp.Body).Decode(&connectionInfo); err != nil {
		return connectionInfo, errors.Wrap(err, "unable to parse API response")
	}

	return connectionInfo, nil
}

func (c Client) CreateSandboxToken(cfg CreateSandboxTokenConfig) (*CreateSandboxTokenResult, error) {
	endpoint := "/mint/api/sandbox_tokens"

	encodedBody, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to encode as JSON")
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(encodedBody))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		msg := extractErrorMessage(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}
		return nil, errors.New(msg)
	}

	result := CreateSandboxTokenResult{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &result, nil
}

// InitiateRun sends a request to Mint for starting a new run
func (c Client) InitiateRun(cfg InitiateRunConfig) (*InitiateRunResult, error) {
	endpoint := "/mint/api/runs"

	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	encodedBody, err := json.Marshal(struct {
		Run InitiateRunConfig `json:"run"`
	}{cfg})
	if err != nil {
		return nil, errors.Wrap(err, "unable to encode as JSON")
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(encodedBody))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		msg := extractErrorMessage(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}

		return nil, errors.New(msg)
	}

	respBody := struct {
		SnakeRunId            string   `json:"run_id"`
		SnakeRunURL           string   `json:"run_url"`
		SnakeTargetedTaskKeys []string `json:"targeted_task_keys"`
		SnakeDefinitionPath   string   `json:"definition_path"`
		SnakeMessage          string   `json:"cli_message"`
		CamelRunId            string   `json:"runId"`
		CamelRunURL           string   `json:"runURL"`
		CamelTargetedTaskKeys []string `json:"targetedTaskKeys"`
		CamelDefinitionPath   string   `json:"definitionPath"`
		CamelMessage          string   `json:"cliMessage"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	if respBody.CamelRunId != "" {
		return &InitiateRunResult{
			RunID:            respBody.CamelRunId,
			RunURL:           respBody.CamelRunURL,
			TargetedTaskKeys: respBody.CamelTargetedTaskKeys,
			DefinitionPath:   respBody.CamelDefinitionPath,
			Message:          respBody.CamelMessage,
		}, nil
	} else {
		return &InitiateRunResult{
			RunID:            respBody.SnakeRunId,
			RunURL:           respBody.SnakeRunURL,
			TargetedTaskKeys: respBody.SnakeTargetedTaskKeys,
			DefinitionPath:   respBody.SnakeDefinitionPath,
			Message:          respBody.SnakeMessage,
		}, nil
	}
}

func (c Client) InitiateDispatch(cfg InitiateDispatchConfig) (*InitiateDispatchResult, error) {
	endpoint := "/mint/api/runs/dispatches"

	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	encodedBody, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to encode as JSON")
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(encodedBody))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		errorStruct := struct {
			Error string `json:"error,omitempty"`
		}{}

		if err := json.NewDecoder(resp.Body).Decode(&errorStruct); err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to call RWX API - %s", resp.Status))
		}

		return nil, errors.New(errorStruct.Error)
	}

	respBody := struct {
		DispatchId string `json:"dispatch_id"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &InitiateDispatchResult{
		DispatchId: respBody.DispatchId,
	}, nil
}

func (c Client) GetDispatch(cfg GetDispatchConfig) (*GetDispatchResult, error) {
	endpoint := fmt.Sprintf("/mint/api/runs/dispatches/%s", cfg.DispatchId)

	req, err := http.NewRequest(http.MethodGet, endpoint, bytes.NewBuffer(make([]byte, 0)))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Unable to call RWX API - %s", resp.Status))
	}

	respBody := struct {
		Status string           `json:"status"`
		Error  string           `json:"error"`
		Runs   []GetDispatchRun `json:"runs"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &GetDispatchResult{
		Status: respBody.Status,
		Error:  respBody.Error,
		Runs:   respBody.Runs,
	}, nil
}

// ObtainAuthCode requests a new one-time-use code to login on a device
func (c Client) ObtainAuthCode(cfg ObtainAuthCodeConfig) (*ObtainAuthCodeResult, error) {
	endpoint := "/api/auth/codes"

	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	encodedBody, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to encode as JSON")
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(encodedBody))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return nil, errors.New(fmt.Sprintf("Unable to call RWX API - %s", resp.Status))
	}

	respBody := ObtainAuthCodeResult{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &respBody, nil
}

// AcquireToken consumes the one-time-use code once authorized
func (c Client) AcquireToken(tokenUrl string) (*AcquireTokenResult, error) {
	req, err := http.NewRequest(http.MethodGet, tokenUrl, bytes.NewBuffer(make([]byte, 0)))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Unable to query the token URL - %s", resp.Status))
	}

	respBody := AcquireTokenResult{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &respBody, nil
}

// Whoami provides details about the authenticated token
func (c Client) Whoami() (*WhoamiResult, error) {
	endpoint := "/api/auth/whoami"

	req, err := http.NewRequest(http.MethodGet, endpoint, bytes.NewBuffer([]byte{}))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Unable to call RWX API - %s", resp.Status))
	}

	respBody := WhoamiResult{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &respBody, nil
}

func (c Client) CreateDocsToken() (*DocsTokenResult, error) {
	endpoint := "/api/auth/docs_token"

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer([]byte{}))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Unable to call RWX API - %s", resp.Status))
	}

	respBody := DocsTokenResult{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &respBody, nil
}

func (c Client) SetSecretsInVault(cfg SetSecretsInVaultConfig) (*SetSecretsInVaultResult, error) {
	endpoint := "/mint/api/vaults/secrets"

	encodedBody, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to encode as JSON")
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(encodedBody))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msg := extractErrorMessage(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}

		return nil, errors.New(msg)
	}

	respBody := SetSecretsInVaultResult{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &respBody, nil
}

func (c Client) GetPackageVersions() (*PackageVersionsResult, error) {
	endpoint := "/mint/api/leaves"

	req, err := http.NewRequest(http.MethodGet, endpoint, bytes.NewBuffer([]byte{}))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msg := extractErrorMessage(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}
		return nil, errors.New(msg)
	}

	respBody := PackageVersionsResult{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &respBody, nil
}

func (c Client) GetPackageDocumentation(packageName string) (*PackageDocumentationResult, error) {
	endpoint := fmt.Sprintf("/mint/api/leaves/%s/documentation", packageName)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != 200 {
		msg := extractErrorMessage(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}
		return nil, errors.New(msg)
	}

	respBody := PackageDocumentationResult{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return &respBody, nil
}

func (c Client) GetDefaultBase() (DefaultBaseResult, error) {
	endpoint := "/mint/api/base/default"
	result := DefaultBaseResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) StartImagePush(cfg StartImagePushConfig) (StartImagePushResult, error) {
	endpoint := "/mint/api/images/pushes"
	result := StartImagePushResult{}

	encodedBody, err := json.Marshal(cfg)
	if err != nil {
		return result, errors.Wrap(err, "unable to encode as JSON")
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(encodedBody))
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) ImagePushStatus(pushID string) (ImagePushStatusResult, error) {
	endpoint := "/mint/api/images/pushes/" + url.PathEscape(pushID)
	result := ImagePushStatusResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) TaskKeyStatus(cfg TaskKeyStatusConfig) (TaskStatusResult, error) {
	endpoint := fmt.Sprintf("/mint/api/runs/%s/task_status?task_key=%s", url.PathEscape(cfg.RunID), url.PathEscape(cfg.TaskKey))
	result := TaskStatusResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) TaskIDStatus(cfg TaskIDStatusConfig) (TaskStatusResult, error) {
	endpoint := fmt.Sprintf("/mint/api/tasks/%s/status", url.PathEscape(cfg.TaskID))
	result := TaskStatusResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) RunStatus(cfg RunStatusConfig) (RunStatusResult, error) {
	var endpoint string
	if cfg.RunID != "" {
		endpoint = fmt.Sprintf("/mint/api/runs/%s?fail_fast=true", url.PathEscape(cfg.RunID))
	} else {
		params := url.Values{}
		params.Set("fail_fast", "true")
		params.Set("branch_name", cfg.BranchName)
		params.Set("repository_name", cfg.RepositoryName)
		endpoint = "/mint/api/runs/latest?" + params.Encode()
	}
	result := RunStatusResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) GetRunPrompt(runID string) (string, error) {
	endpoint := fmt.Sprintf("/mint/api/runs/%s/prompt", url.PathEscape(runID))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return "", errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg := extractErrorMessage(resp.Body)
		if errMsg == "" {
			errMsg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}
		if resp.StatusCode == http.StatusNotFound {
			return "", errors.Wrap(ErrNotFound, errMsg)
		}
		return "", errors.New(errMsg)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "unable to read response body")
	}

	return string(body), nil
}

func (c Client) GetLogDownloadRequest(taskId string) (LogDownloadRequestResult, error) {
	endpoint := fmt.Sprintf("/mint/api/log_downloads/%s", url.PathEscape(taskId))
	result := LogDownloadRequestResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) DownloadLogs(request LogDownloadRequestResult, maxRetryDurationSeconds ...int) ([]byte, error) {
	maxRetryDuration := 30 * time.Second
	if len(maxRetryDurationSeconds) > 0 && maxRetryDurationSeconds[0] > 0 {
		maxRetryDuration = time.Duration(maxRetryDurationSeconds[0]) * time.Second
	}
	const initialBackoff = 1 * time.Second

	startTime := time.Now()
	backoff := initialBackoff
	attempt := 0

	var lastErr error

	for {
		attempt++

		// need to recreate for each attempt since body readers are consumed
		var req *http.Request
		var err error

		if request.Contents != nil {
			// POST approach, for zip files (group tasks)
			formData := url.Values{}
			formData.Set("token", request.Token)
			formData.Set("filename", request.Filename)
			formData.Set("contents", *request.Contents)
			encodedBody := formData.Encode()

			req, err = http.NewRequest(http.MethodPost, request.URL, strings.NewReader(encodedBody))
			if err != nil {
				return nil, errors.Wrap(err, "unable to create new HTTP request")
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/octet-stream")
		} else {
			// GET approach, for single log files
			req, err = http.NewRequest(http.MethodGet, request.URL, nil)
			if err != nil {
				return nil, errors.Wrap(err, "unable to create new HTTP request")
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", request.Token))
			req.Header.Set("Accept", "application/octet-stream")
		}

		// Use http.DefaultClient directly since the logs will come from a task server URL rather than Cloud
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = errors.Wrap(err, "HTTP request failed")

			if time.Since(startTime) >= maxRetryDuration {
				return nil, errors.Wrapf(lastErr, "failed after %d attempts over %v", attempt, time.Since(startTime).Round(time.Second))
			}

			time.Sleep(backoff)
			backoff *= 2
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()
			logBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, errors.Wrap(err, "unable to read response body")
			}
			return logBytes, nil
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Don't retry on 4xx errors
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			errMsg := extractErrorMessage(bytes.NewReader(bodyBytes))
			if errMsg == "" {
				errMsg = fmt.Sprintf("Unable to download logs - %s", resp.Status)
			}
			return nil, errors.New(errMsg)
		}

		// Retry on 5xx errors - task server may be waking up
		errMsg := extractErrorMessage(bytes.NewReader(bodyBytes))
		if errMsg == "" {
			errMsg = fmt.Sprintf("Unable to download logs - %s", resp.Status)
		}
		lastErr = errors.New(errMsg)

		if time.Since(startTime) >= maxRetryDuration {
			return nil, errors.Wrapf(lastErr, "failed after %d attempts over %v", attempt, time.Since(startTime).Round(time.Second))
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
}

func (c Client) GetAllArtifactDownloadRequests(taskId string) ([]ArtifactDownloadRequestResult, error) {
	endpoint := fmt.Sprintf("/mint/api/tasks/%s/artifact_downloads", url.PathEscape(taskId))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg := extractErrorMessage(resp.Body)
		if errMsg == "" {
			errMsg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.Wrap(ErrNotFound, errMsg)
		}
		return nil, errors.New(errMsg)
	}

	var results []ArtifactDownloadRequestResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, errors.Wrap(err, "unable to parse API response")
	}

	return results, nil
}

func (c Client) GetArtifactDownloadRequest(taskId, artifactKey string) (ArtifactDownloadRequestResult, error) {
	endpoint := fmt.Sprintf("/mint/api/tasks/%s/artifact_downloads/%s", url.PathEscape(taskId), url.PathEscape(artifactKey))
	result := ArtifactDownloadRequestResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c Client) DownloadArtifact(request ArtifactDownloadRequestResult) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, request.URL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Accept", "application/octet-stream")

	// Use http.DefaultClient directly since the artifact will come from storage (S3, etc.)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		artifactBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "unable to read response body")
		}
		return artifactBytes, nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	errMsg := extractErrorMessage(bytes.NewReader(bodyBytes))
	if errMsg == "" {
		errMsg = fmt.Sprintf("Unable to download artifact - %s", resp.Status)
	}
	return nil, errors.New(errMsg)
}

func (c Client) CancelRun(runID, scopedToken string) error {
	if runID == "" {
		return errors.New("missing runID")
	}

	endpoint := fmt.Sprintf("/mint/api/runs/%s/cancel", url.PathEscape(runID))
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return errors.Wrap(err, "unable to create new HTTP request")
	}

	if scopedToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", scopedToken))
	}

	resp, err := c.RoundTrip(req)
	if err != nil {
		return errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msg := extractErrorMessage(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
		}
		return errors.New(msg)
	}

	return nil
}

func (c Client) GetSandboxInitTemplate() (SandboxInitTemplateResult, error) {
	endpoint := "/mint/api/sandbox_init_template"
	result := SandboxInitTemplateResult{}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeResponseJSON(resp, &result); err != nil {
		return result, err
	}

	return result, nil
}

func decodeResponseJSON(resp *http.Response, result any) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if result == nil {
			return nil
		}

		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return errors.Wrap(err, "unable to parse API response")
		}
		return nil
	}

	errMsg := extractErrorMessage(resp.Body)
	if errMsg == "" {
		errMsg = fmt.Sprintf("Unable to call RWX API - %s", resp.Status)
	}

	if resp.StatusCode == http.StatusNotFound {
		return errors.Wrap(ErrNotFound, errMsg)
	}

	return errors.New(errMsg)
}

type ErrorMessage struct {
	Message    string                `json:"message"`
	StackTrace []messages.StackEntry `json:"stack_trace,omitempty"`
	Frame      string                `json:"frame"`
	Advice     string                `json:"advice"`
}

// extractErrorMessage is a small helper function for parsing an API error message
func extractErrorMessage(reader io.Reader) string {
	errorStruct := struct {
		Error         string         `json:"error,omitempty"`
		ErrorMessages []ErrorMessage `json:"error_messages,omitempty"`
	}{}

	if err := json.NewDecoder(reader).Decode(&errorStruct); err != nil {
		return ""
	}

	if len(errorStruct.ErrorMessages) > 0 {
		var message strings.Builder
		for _, errorMessage := range errorStruct.ErrorMessages {
			message.WriteString("\n\n")
			message.WriteString(messages.FormatUserMessage(errorMessage.Message, errorMessage.Frame, errorMessage.StackTrace, errorMessage.Advice))
		}

		return message.String()
	}

	// Fallback to Error field
	if errorStruct.Error != "" {
		return errorStruct.Error
	}

	// Fallback to an empty string
	return ""
}
