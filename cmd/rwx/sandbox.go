package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

var sandboxCmd = &cobra.Command{
	GroupID: "execution",
	Use:     "sandbox",
	Short:   "Run commands in persistent sandboxes",
	Hidden:  true,
}

var sandboxStartCmd = &cobra.Command{
	Use:   "start [config-file]",
	Short: "Start a sandbox without executing a command",
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile := ".rwx/sandbox.yml"
		if len(args) > 0 {
			configFile = args[0]
		}

		useJson := useJsonOutput()

		// Check for existing active sandbox (skip if --id is provided)
		if sandboxRunID == "" {
			existing, err := service.CheckExistingSandbox(configFile)
			if err != nil {
				return err
			}

			if existing.Exists && existing.Active {
				// Prompt user for what to do
				fmt.Fprintf(os.Stdout, "An active sandbox already exists for this directory and branch:\n")
				fmt.Fprintf(os.Stdout, "  Run ID: %s\n", existing.RunID)
				fmt.Fprintf(os.Stdout, "  URL: %s\n\n", existing.RunURL)

				prompt := promptui.Select{
					Label: "What would you like to do",
					Items: []string{"Continue with existing sandbox", "Stop and start a new sandbox"},
				}

				idx, _, err := prompt.Run()
				if err != nil {
					return err
				}

				if idx == 0 {
					// Continue with existing
					if sandboxOpen && existing.RunURL != "" {
						if openErr := open.Run(existing.RunURL); openErr != nil {
							fmt.Fprintf(os.Stderr, "Failed to open browser.\n")
						}
					}

					if useJson {
						result := cli.StartSandboxResult{
							RunID:      existing.RunID,
							RunURL:     existing.RunURL,
							ConfigFile: existing.ConfigFile,
						}
						jsonOutput, err := json.Marshal(result)
						if err != nil {
							return err
						}
						fmt.Println(string(jsonOutput))
					} else {
						fmt.Fprintf(os.Stdout, "Using existing sandbox: %s\n", existing.RunID)
					}
					return nil
				}

				// User chose to reset - stop existing and continue to start new
				_, err = service.StopSandbox(cli.StopSandboxConfig{
					RunID: existing.RunID,
					Json:  useJson,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to stop existing sandbox: %v\n", err)
				}
			}
		}

		result, err := service.StartSandbox(cli.StartSandboxConfig{
			ConfigFile:   configFile,
			RunID:        sandboxRunID,
			RwxDirectory: sandboxRwxDir,
			Json:         useJson,
			Wait:         sandboxWait,
		})

		// Open browser if we have a URL, even if there was an error
		if sandboxOpen && result != nil && result.RunURL != "" {
			if openErr := open.Run(result.RunURL); openErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to open browser.\n")
			}
		}

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

var sandboxExecCmd = &cobra.Command{
	Use:   "exec [config-file] -- <command>",
	Short: "Execute a command in a sandbox",
	Args:  cobra.ArbitraryArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get command args after --
		dashIndex := cmd.ArgsLenAtDash()
		var command []string
		var configFile string

		if dashIndex < 0 {
			// No -- found, error
			return fmt.Errorf("No command specified. Usage: rwx sandbox exec [config-file] -- <command>")
		}

		// Args before -- are config file (optional)
		if dashIndex > 0 {
			configFile = args[0]
		}

		// Args after -- are the command
		if dashIndex < len(args) {
			command = args[dashIndex:]
		}

		if len(command) == 0 {
			return fmt.Errorf("No command specified. Usage: rwx sandbox exec [config-file] -- <command>")
		}

		useJson := useJsonOutput()
		result, err := service.ExecSandbox(cli.ExecSandboxConfig{
			ConfigFile:   configFile,
			Command:      command,
			RunID:        sandboxRunID,
			RwxDirectory: sandboxRwxDir,
			Json:         useJson,
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

		if sandboxOpen {
			if err := open.Run(result.RunURL); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open browser.\n")
			}
		}

		os.Exit(result.ExitCode)
		return nil
	},
}

var sandboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sandbox sessions with status",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		useJson := useJsonOutput()
		result, err := service.ListSandboxes(cli.ListSandboxesConfig{
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

var sandboxStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a sandbox session",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		useJson := useJsonOutput()
		result, err := service.StopSandbox(cli.StopSandboxConfig{
			RunID: sandboxRunID,
			All:   sandboxStopAll,
			Json:  useJson,
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

var sandboxResetCmd = &cobra.Command{
	Use:   "reset [config-file]",
	Short: "Stop and restart a sandbox",
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return requireAccessToken()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile := ".rwx/sandbox.yml"
		if len(args) > 0 {
			configFile = args[0]
		}

		useJson := useJsonOutput()
		result, err := service.ResetSandbox(cli.ResetSandboxConfig{
			ConfigFile:   configFile,
			RwxDirectory: sandboxRwxDir,
			Json:         useJson,
			Wait:         sandboxWait,
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

		if sandboxOpen {
			if err := open.Run(result.RunURL); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open browser.\n")
			}
		}

		return nil
	},
}

var (
	sandboxRunID   string
	sandboxStopAll bool
	sandboxRwxDir  string
	sandboxOpen    bool
	sandboxWait    bool
)

func init() {
	sandboxCmd.AddCommand(sandboxStartCmd)
	sandboxCmd.AddCommand(sandboxExecCmd)
	sandboxCmd.AddCommand(sandboxListCmd)
	sandboxCmd.AddCommand(sandboxStopCmd)
	sandboxCmd.AddCommand(sandboxResetCmd)

	// start flags
	sandboxStartCmd.Flags().StringVarP(&sandboxRwxDir, "dir", "d", "", "RWX directory")
	sandboxStartCmd.Flags().StringVar(&sandboxRunID, "id", "", "Use specific run ID")
	sandboxStartCmd.Flags().BoolVar(&sandboxOpen, "open", false, "Open the run in a browser")
	sandboxStartCmd.Flags().BoolVar(&sandboxWait, "wait", false, "Wait for sandbox to be ready")

	// exec flags
	sandboxExecCmd.Flags().StringVarP(&sandboxRwxDir, "dir", "d", "", "RWX directory")
	sandboxExecCmd.Flags().StringVar(&sandboxRunID, "id", "", "Use specific run ID")
	sandboxExecCmd.Flags().BoolVar(&sandboxOpen, "open", false, "Open the run in a browser")

	// stop flags
	sandboxStopCmd.Flags().StringVar(&sandboxRunID, "id", "", "Stop specific sandbox by run ID")
	sandboxStopCmd.Flags().BoolVar(&sandboxStopAll, "all", false, "Stop all sandboxes")

	// reset flags
	sandboxResetCmd.Flags().StringVarP(&sandboxRwxDir, "dir", "d", "", "RWX directory")
	sandboxResetCmd.Flags().BoolVar(&sandboxOpen, "open", false, "Open the run in a browser")
	sandboxResetCmd.Flags().BoolVar(&sandboxWait, "wait", false, "Wait for sandbox to be ready")
}
