package main

import (
	"fmt"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var resolveCmd = &cobra.Command{
	GroupID: "definitions",
	Short:   "Resolve and add versions for base images and RWX packages",
	Use:     "resolve [flags] [files...]",
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
	ResolvePackagesJson   bool
	ResolvePackagesOutput string
	ResolveBaseJson       bool
	ResolveBaseOutput     string

	resolveBaseCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return resolveBase(args)
		},
		Short: "Add a base image to RWX run configurations that do not have one",
		Long: "Add a base image to RWX run configurations that do not have one.\n" +
			"Updates all top-level YAML files in .rwx that are missing a 'base' to include one.",
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
	useJson := ResolveBaseOutput == "json" || ResolveBaseJson
	base, err := service.InsertBase(cli.InsertBaseConfig{
		Files:        files,
		RwxDirectory: RwxDirectory,
		Json:         useJson,
	})
	if err != nil {
		return err
	}
	if !useJson && base.HasChanges() {
		fmt.Println()
	}
	return nil
}

func resolvePackages(files []string) error {
	useJson := ResolvePackagesOutput == "json" || ResolvePackagesJson
	_, err := service.ResolvePackages(cli.ResolvePackagesConfig{
		Files:               files,
		RwxDirectory:        RwxDirectory,
		LatestVersionPicker: cli.PickLatestMajorVersion,
		Json:                useJson,
	})
	return err
}

func init() {
	resolveBaseCmd.Flags().BoolVar(&ResolveBaseJson, "json", false, "output JSON instead of text")
	_ = resolveBaseCmd.Flags().MarkHidden("json")
	resolveBaseCmd.Flags().StringVar(&ResolveBaseOutput, "output", "text", "output format: text or json")
	addRwxDirFlag(resolveBaseCmd)

	resolvePackagesCmd.Flags().BoolVar(&ResolvePackagesJson, "json", false, "output JSON instead of text")
	_ = resolvePackagesCmd.Flags().MarkHidden("json")
	resolvePackagesCmd.Flags().StringVar(&ResolvePackagesOutput, "output", "text", "output format: text or json")
	addRwxDirFlag(resolvePackagesCmd)

	resolveCmd.AddCommand(resolveBaseCmd)
	resolveCmd.AddCommand(resolvePackagesCmd)
	addRwxDirFlag(resolveCmd)
}
