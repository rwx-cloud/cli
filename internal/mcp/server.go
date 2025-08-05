package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rwx-cloud/cli/internal/accesstoken"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/versions"
)

type ServeConfig struct {
	AccessToken        string
	Host               string
	AccessTokenBackend accesstoken.Backend
}

func Serve(ctx context.Context, config ServeConfig) error {
	apiClient, err := api.NewClient(api.Config{
		AccessToken:        config.AccessToken,
		Host:               config.Host,
		AccessTokenBackend: config.AccessTokenBackend,
	})
	if err != nil {
		return err
	}

	server := NewServer(ServerConfig{APIClient: apiClient})
	return server.Run(ctx, mcp.NewStdioTransport())
}

type Server struct {
	ms  *mcp.Server
	api APIClient
}

type ServerConfig struct {
	APIClient APIClient
}

func NewServer(config ServerConfig) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "rwx-cli-mcp-server",
		Version: versions.GetCliCurrentVersion().String(),
	}, &mcp.ServerOptions{})

	server := &Server{ms: mcpServer, api: config.APIClient}
	server.addTools()

	return server
}

func (s *Server) Run(ctx context.Context, transport mcp.Transport) error {
	return s.ms.Run(ctx, transport)
}

func (s *Server) addTools() {
	mcp.AddTool(s.ms, &mcp.Tool{
		Name:        "get_run_test_failures",
		Description: "Get the list of failed tests for the given run(s)",
	}, s.getRunTestFailures)
}

type GetRunFailedTestsInput struct {
	RunUrls []string `json:"run_urls" jsonschema:"The URLs or IDs of the run"`
}

func (s *Server) getRunTestFailures(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[GetRunFailedTestsInput]) (*mcp.CallToolResult, error) {
	res, err := s.api.McpGetRunTestFailures(api.McpGetRunTestFailuresRequest{RunUrls: params.Arguments.RunUrls})
	if err != nil {
		return nil, err
	}

	return mcpToolTextResult(res.Text), nil
}

func mcpToolTextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}
}
