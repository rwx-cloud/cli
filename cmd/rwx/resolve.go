package main

import (
	"fmt"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var resolveCmd = &cobra.Command{
	Short: "Resolve and add versions for base layers and RWX packages",
	Use:   "resolve [flags] [files...]",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			switch args[0] {
			case "base":
				return resolveBase(args[1:])
			case "packages":
				return resolvePackages(args[1:])
			}
		}

		err := resolveBase(args)
		if err != nil {
			return err
		}
		return resolvePackages(args)
	},
}

var (
	resolveBaseCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return resolveBase(args)
		},
		Short: "Add a base layer to RWX run configurations that do not have one",
		Long: "Add a base layer to RWX run configurations that do not have one.\n" +
			"Updates all top-level YAML files in .rwx that are missing a 'base' to include one.\n" +
			"The best match will be found based on the provided flags. If no flags are provided,\n" +
			"it will use the current default base layer.",
		Use: "base [flags] [files...]",
	}

	resolvePackagesCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return resolvePackages(args)
		},
		Short: "Add the latest version to all package invocations that do not have one",
		Long: "Add the latest version to all package invocations that do not have one.\n" +
			"Updates all top-level YAML files in .rwx that 'call' a package without a version\n" +
			"to use the latest version.",
		Use: "packages [flags] [files...]",
	}
)

func resolveBase(files []string) error {
	base, err := service.ResolveBase(cli.ResolveBaseConfig{
		Files:        files,
		RwxDirectory: RwxDirectory,
	})
	if err != nil {
		return err
	}
	if base.HasChanges() {
		fmt.Println()
	}
	return nil
}

func resolvePackages(files []string) error {
	_, err := service.ResolvePackages(cli.ResolvePackagesConfig{
		Files:               files,
		RwxDirectory:        RwxDirectory,
		LatestVersionPicker: cli.PickLatestMajorVersion,
	})
	return err
}

func init() {
	addRwxDirFlag(resolveBaseCmd)

	addRwxDirFlag(resolvePackagesCmd)

	resolveCmd.AddCommand(resolveBaseCmd)
	resolveCmd.AddCommand(resolvePackagesCmd)
	addRwxDirFlag(resolveCmd)
}
