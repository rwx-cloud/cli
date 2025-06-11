package main

import (
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Short: "Update versions for base layers and Mint leaves",
	Use:   "update [flags] [files...]",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			switch args[0] {
			case "base":
				return updateBase(args[1:])
			case "leaves":
				return updateLeaves(args[1:])
			}
		}

		err := updateBase(args)
		if err != nil {
			return err
		}
		return updateLeaves(args)
	},
}

var (
	AllowMajorVersionChange bool

	updateBaseCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateBase(args)
		},
		Short: "Update all base layers to their latest (minor) version",
		Long: "Update all base layers to their latest (minor) version.\n" +
			"Takes a list of files as arguments, or updates all toplevel YAML files in .mint if no files are given.",
		Use: "base [flags] [files...]",
	}

	updateLeavesCmd = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateLeaves(args)
		},
		Short: "Update all leaves to their latest (minor) version",
		Long: "Update all leaves to their latest (minor) version.\n" +
			"Takes a list of files as arguments, or updates all toplevel YAML files in .mint if no files are given.",
		Use: "leaves [flags] [files...]",
	}
)

func updateBase(files []string) error {
	_, err := service.UpdateBase(cli.UpdateBaseConfig{
		Files:        files,
		RwxDirectory: RwxDirectory,
	})
	return err
}

func updateLeaves(files []string) error {
	replacementVersionPicker := cli.PickLatestMinorVersion
	if AllowMajorVersionChange {
		replacementVersionPicker = cli.PickLatestMajorVersion
	}

	return service.UpdateLeaves(cli.UpdateLeavesConfig{
		Files:                    files,
		RwxDirectory:             RwxDirectory,
		ReplacementVersionPicker: replacementVersionPicker,
	})
}

func init() {
	addRwxDirFlag(updateBaseCmd)

	updateLeavesCmd.Flags().BoolVar(&AllowMajorVersionChange, "allow-major-version-change", false, "update leaves to the latest major version")
	addRwxDirFlag(updateLeavesCmd)

	updateCmd.Flags().BoolVar(&AllowMajorVersionChange, "allow-major-version-change", false, "update to the latest major version")
	updateCmd.AddCommand(updateBaseCmd)
	updateCmd.AddCommand(updateLeavesCmd)
	addRwxDirFlag(updateCmd)
}
