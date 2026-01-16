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
	PackagesUpdateJson              bool
	PackagesUpdateOutput            string

	packagesUpdateCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			replacementVersionPicker := cli.PickLatestMinorVersion
			if PackagesAllowMajorVersionChange {
				replacementVersionPicker = cli.PickLatestMajorVersion
			}

			useJson := PackagesUpdateOutput == "json" || PackagesUpdateJson
			return service.UpdatePackages(cli.UpdatePackagesConfig{
				Files:                    args,
				RwxDirectory:             RwxDirectory,
				ReplacementVersionPicker: replacementVersionPicker,
				Json:                     useJson,
			})
		},
		Short: "Update all packages to their latest (minor) version",
		Long: "Update all packages to their latest (minor) version.\n" +
			"Takes a list of files as arguments, or updates all toplevel YAML files in .rwx if no files are given.",
		Use: "update [flags] [file...]",
	}
)

func init() {
	packagesUpdateCmd.Flags().BoolVar(&PackagesAllowMajorVersionChange, "allow-major-version-change", false, "update packages to the latest major version")
	packagesUpdateCmd.Flags().BoolVar(&PackagesUpdateJson, "json", false, "output JSON instead of text")
	_ = packagesUpdateCmd.Flags().MarkHidden("json")
	packagesUpdateCmd.Flags().StringVar(&PackagesUpdateOutput, "output", "text", "output format: text or json")
	addRwxDirFlag(packagesUpdateCmd)
	packagesCmd.AddCommand(packagesUpdateCmd)
}
