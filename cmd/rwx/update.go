package main

import (
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	GroupID: "commands",
	Short:   "Update versions for base layers and RWX packages",
	Use:     "update [flags] [files...]",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			switch args[0] {
			case "base":
				return updateBase(args[1:])
			case "packages":
				return updatePackages(args[1:])
			}
		}

		err := updateBase(args)
		if err != nil {
			return err
		}
		return updatePackages(args)
	},
}

var (
	AllowMajorVersionChange bool

	updateBaseCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateBase(args)
		},
		Short: "Add a base image to RWX run configurations that do not have one",
		Long: "Add a base image to RWX run configurations that do not have one.\n" +
			"Updates all top-level YAML files in .rwx that are missing a 'base' to include one.",
		Use: "base [flags] [files...]",
	}

	updatePackagesCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return updatePackages(args)
		},
		Short: "Update all packages to their latest (minor) version",
		Long: "Update all packages to their latest (minor) version.\n" +
			"Takes a list of files as arguments, or updates all toplevel YAML files in .rwx if no files are given.",
		Use: "packages [flags] [files...]",
	}
)

func updateBase(files []string) error {
	_, err := service.InsertBase(cli.InsertBaseConfig{
		Files:        files,
		RwxDirectory: RwxDirectory,
	})
	return err
}

func updatePackages(files []string) error {
	replacementVersionPicker := cli.PickLatestMinorVersion
	if AllowMajorVersionChange {
		replacementVersionPicker = cli.PickLatestMajorVersion
	}

	return service.UpdatePackages(cli.UpdatePackagesConfig{
		Files:                    files,
		RwxDirectory:             RwxDirectory,
		ReplacementVersionPicker: replacementVersionPicker,
	})
}

func init() {
	addRwxDirFlag(updateBaseCmd)

	updatePackagesCmd.Flags().BoolVar(&AllowMajorVersionChange, "allow-major-version-change", false, "update packages to the latest major version")
	addRwxDirFlag(updatePackagesCmd)

	updateCmd.Flags().BoolVar(&AllowMajorVersionChange, "allow-major-version-change", false, "update to the latest major version")
	updateCmd.AddCommand(updateBaseCmd)
	updateCmd.AddCommand(updatePackagesCmd)
	addRwxDirFlag(updateCmd)
}
