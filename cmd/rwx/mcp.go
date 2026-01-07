package main

import (
	"github.com/rwx-cloud/cli/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	mcpCmd = &cobra.Command{
		GroupID: "commands",
		Use:     "mcp",
		Short:   "MCP (Model Context Protocol) related commands",
	}

	mcpServeCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start an MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := mcp.ServeConfig{
				AccessToken:        AccessToken,
				Host:               rwxHost,
				AccessTokenBackend: accessTokenBackend,
			}
			return mcp.Serve(cmd.Context(), config)
		},
	}
)

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
}
