package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	GroupID: "setup",
	Use:     "docs",
	Short:   "Search and read RWX documentation",
}

var docsSearchLimit int

var docsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search RWX documentation",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		useJson := useJsonOutput()

		result, err := service.DocsSearch(cli.DocsSearchConfig{
			Query:       query,
			Limit:       docsSearchLimit,
			Json:        useJson,
			StdoutIsTTY: service.StdoutIsTTY,
		})
		if err != nil {
			return err
		}

		if useJson {
			jsonOutput, err := json.Marshal(result)
			if err != nil {
				return err
			}
			fmt.Println(string(jsonOutput))
		}

		return nil
	},
}

var docsPullCmd = &cobra.Command{
	Use:   "pull <url-or-path>",
	Short: "Fetch an RWX documentation article as markdown",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		useJson := useJsonOutput()

		result, err := service.DocsPull(cli.DocsPullConfig{
			URL:  args[0],
			Json: useJson,
		})
		if err != nil {
			return err
		}

		if useJson {
			jsonOutput, err := json.Marshal(result)
			if err != nil {
				return err
			}
			fmt.Println(string(jsonOutput))
		}

		return nil
	},
}

func init() {
	docsCmd.AddCommand(docsSearchCmd)
	docsCmd.AddCommand(docsPullCmd)

	docsSearchCmd.Flags().IntVar(&docsSearchLimit, "limit", 5, "Maximum number of search results")
}
