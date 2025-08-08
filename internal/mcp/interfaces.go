package mcp

import "github.com/rwx-cloud/cli/internal/api"

type APIClient interface {
	McpGetRunTestFailures(api.McpGetRunTestFailuresRequest) (*api.McpTextResult, error)
}

var _ APIClient = api.Client{}
