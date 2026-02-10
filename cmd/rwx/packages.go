package main

import (
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/spf13/cobra"
)

var packagesCmd = &cobra.Command{
	GroupID: "definitions",
	Hidden:  true,
	Short:   "Manage RWX packages",
	Use:     "packages",
}

var (
	PackagesAllowMajorVersionChange bool

	packagesListCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := service.ListPackages(cli.ListPackagesConfig{
				Json: useJsonOutput(),
			})
			return err
		},
		Short: "List all available packages and their latest versions",
		Use:   "list",
		Args:  cobra.NoArgs,
	}

	packagesShowCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := service.ShowPackage(cli.ShowPackageConfig{
				PackageName: args[0],
				Json:        useJsonOutput(),
			})
			return err
		},
		Short: "Show details for a package",
		Use:   "show [flags] <package-name>",
	}

	packagesUpdateCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			replacementVersionPicker := cli.PickLatestMinorVersion
			if PackagesAllowMajorVersionChange {
				replacementVersionPicker = cli.PickLatestMajorVersion
			}

			useJson := useJsonOutput()
			_, err := service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    args,
				RwxDirectory:             RwxDirectory,
				ReplacementVersionPicker: replacementVersionPicker,
				Json:                     useJson,
			})
			return err
		},
		Short: "Update all packages to their latest (minor) version",
		Long: "Update all packages to their latest (minor) version.\n" +
			"Takes a list of files as arguments, or updates all toplevel YAML files in .rwx if no files are given.",
		Use: "update [flags] [file...]",
	}
)

func init() {
	packagesUpdateCmd.Flags().BoolVar(&PackagesAllowMajorVersionChange, "allow-major-version-change", false, "update packages to the latest major version")
	addRwxDirFlag(packagesUpdateCmd)
	packagesCmd.AddCommand(packagesListCmd)
	packagesCmd.AddCommand(packagesShowCmd)
	packagesCmd.AddCommand(packagesUpdateCmd)
}
