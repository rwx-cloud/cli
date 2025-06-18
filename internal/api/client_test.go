package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/versions"
	"github.com/stretchr/testify/require"
)

func TestAPIClient_InitiateRun(t *testing.T) {
	t.Run("prefixes the endpoint with the base path and parses camelcase responses", func(t *testing.T) {
		body := struct {
			RunID            string   `json:"runId"`
			RunURL           string   `json:"runUrl"`
			TargetedTaskKeys []string `json:"targetedTaskKeys"`
			DefinitionPath   string   `json:"definitionPath"`
		}{
			RunID:            "123",
			RunURL:           "https://cloud.rwx.com/mint/org/runs/123",
			TargetedTaskKeys: []string{},
			DefinitionPath:   "foo",
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/mint/api/runs", req.URL.Path)
			return &http.Response{
				Status:     "201 Created",
				StatusCode: 201,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
				Header:     http.Header{"X-Mint-Cli-Latest-Version": []string{"1000000.0.0"}},
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		initRunConfig := api.InitiateRunConfig{
			InitializationParameters: []api.InitializationParameter{},
			TaskDefinitions: []api.RwxDirectoryEntry{
				{Path: "foo", FileContents: "echo 'bar'", Permissions: 0o644, Type: "file"},
			},
			TargetedTaskKeys: []string{},
			UseCache:         false,
		}

		result, err := c.InitiateRun(initRunConfig)
		require.NoError(t, err)
		require.Equal(t, "123", result.RunId)

		// This works as long as this is the only test we're setting the latest version header.
		require.Equal(t, "1000000.0.0", versions.GetCliLatestVersion().String())
		require.True(t, versions.NewVersionAvailable())
	})

	t.Run("prefixes the endpoint with the base path and parses snakecase responses", func(t *testing.T) {
		body := struct {
			RunID            string   `json:"run_id"`
			RunURL           string   `json:"run_url"`
			TargetedTaskKeys []string `json:"targeted_task_keys"`
			DefinitionPath   string   `json:"definition_path"`
		}{
			RunID:            "123",
			RunURL:           "https://cloud.rwx.com/mint/org/runs/123",
			TargetedTaskKeys: []string{},
			DefinitionPath:   "foo",
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/mint/api/runs", req.URL.Path)
			return &http.Response{
				Status:     "201 Created",
				StatusCode: 201,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		initRunConfig := api.InitiateRunConfig{
			InitializationParameters: []api.InitializationParameter{},
			TaskDefinitions: []api.RwxDirectoryEntry{
				{Path: "foo", FileContents: "echo 'bar'", Permissions: 0o644, Type: "file"},
			},
			TargetedTaskKeys: []string{},
			UseCache:         false,
		}

		result, err := c.InitiateRun(initRunConfig)
		require.NoError(t, err)
		require.Equal(t, "123", result.RunId)
	})
}

func TestAPIClient_ObtainAuthCode(t *testing.T) {
	t.Run("builds the request", func(t *testing.T) {
		body := struct {
			AuthorizationUrl string `json:"authorization_url"`
			TokenUrl         string `json:"token_url"`
		}{
			AuthorizationUrl: "https://cloud.rwx.com/_/auth/code?code=foobar",
			TokenUrl:         "https://cloud.rwx.com/api/auth/codes/code-uuid/token",
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/api/auth/codes", req.URL.Path)
			return &http.Response{
				Status:     "201 Created",
				StatusCode: 201,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		obtainAuthCodeConfig := api.ObtainAuthCodeConfig{
			Code: api.ObtainAuthCodeCode{
				DeviceName: "some-device",
			},
		}

		_, err := c.ObtainAuthCode(obtainAuthCodeConfig)
		require.NoError(t, err)
	})
}

func TestAPIClient_AcquireToken(t *testing.T) {
	t.Run("builds the request using the supplied url", func(t *testing.T) {
		body := struct {
			State string `json:"state"`
			Token string `json:"token"`
		}{
			State: "authorized",
			Token: "some-token",
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			expected, err := url.Parse("https://cloud.rwx.com/api/auth/codes/some-uuid/token")
			require.NoError(t, err)
			require.Equal(t, expected, req.URL)
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		_, err := c.AcquireToken("https://cloud.rwx.com/api/auth/codes/some-uuid/token")
		require.NoError(t, err)
	})
}

func TestAPIClient_Whoami(t *testing.T) {
	t.Run("makes the request", func(t *testing.T) {
		email := "some-email@example.com"
		body := struct {
			OrganizationSlug string  `json:"organization_slug"`
			TokenKind        string  `json:"token_kind"`
			UserEmail        *string `json:"user_email,omitempty"`
		}{
			OrganizationSlug: "some-org",
			TokenKind:        "personal_access_token",
			UserEmail:        &email,
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/api/auth/whoami", req.URL.Path)
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		_, err := c.Whoami()
		require.NoError(t, err)
	})
}

func TestAPIClient_SetSecretsInVault(t *testing.T) {
	t.Run("makes the request", func(t *testing.T) {
		body := api.SetSecretsInVaultConfig{
			VaultName: "default",
			Secrets:   []api.Secret{{Name: "ABC", Secret: "123"}},
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/mint/api/vaults/secrets", req.URL.Path)
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		_, err := c.SetSecretsInVault(body)
		require.NoError(t, err)
	})
}

func TestAPIClient_InitiateDispatch(t *testing.T) {
	t.Run("builds the request and parses the response", func(t *testing.T) {
		body := struct {
			DispatchId string `json:"dispatch_id"`
		}{
			DispatchId: "dispatch-123",
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/mint/api/runs/dispatches", req.URL.Path)
			require.Equal(t, http.MethodPost, req.Method)
			return &http.Response{
				Status:     "201 Created",
				StatusCode: 201,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		dispatchConfig := api.InitiateDispatchConfig{
			DispatchKey: "test-dispatch-key",
			Params:      map[string]string{"key1": "value1"},
			Ref:         "main",
			Title:       "Test Dispatch",
		}

		result, err := c.InitiateDispatch(dispatchConfig)
		require.NoError(t, err)
		require.Equal(t, "dispatch-123", result.DispatchId)
	})
}

func TestAPIClient_GetDispatch(t *testing.T) {
	t.Run("builds the request and parses the response", func(t *testing.T) {
		body := struct {
			Status string               `json:"status"`
			Error  string               `json:"error"`
			Runs   []api.GetDispatchRun `json:"runs"`
		}{
			Status: "ready",
			Error:  "",
			Runs:   []api.GetDispatchRun{{RunId: "run-123", RunUrl: "https://example.com/run-123"}},
		}
		bodyBytes, _ := json.Marshal(body)

		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/mint/api/runs/dispatches/dispatch-123", req.URL.Path)
			require.Equal(t, http.MethodGet, req.Method)
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		dispatchConfig := api.GetDispatchConfig{
			DispatchId: "dispatch-123",
		}

		result, err := c.GetDispatch(dispatchConfig)
		require.NoError(t, err)
		require.Equal(t, "ready", result.Status)
		require.Len(t, result.Runs, 1)
		require.Equal(t, "run-123", result.Runs[0].RunId)
		require.Equal(t, "https://example.com/run-123", result.Runs[0].RunUrl)
	})
}

func TestAPIClient_ResolveBaseLayer(t *testing.T) {
	t.Run("builds the request and parses the response", func(t *testing.T) {
		roundTrip := func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/mint/api/base_layers/resolve", req.URL.Path)
			require.Equal(t, http.MethodPost, req.Method)
			reqBody, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Contains(t, string(reqBody), "gentoo 99")

			body := `{"os": "gentoo 99", "tag": "1.2", "arch": "quantum"}`
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader([]byte(body))),
			}, nil
		}

		c := api.NewClientWithRoundTrip(roundTrip)

		resolveConfig := api.ResolveBaseLayerConfig{
			Os: "gentoo 99",
		}

		result, err := c.ResolveBaseLayer(resolveConfig)
		require.NoError(t, err)
		require.Equal(t, "gentoo 99", result.Os)
		require.Equal(t, "1.2", result.Tag)
		require.Equal(t, "quantum", result.Arch)
	})
}
