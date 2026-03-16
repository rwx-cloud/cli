package main

import "github.com/spf13/cobra"

var runsCmd *cobra.Command

func init() {
	aliasShort := "Alias for rwx results"

	runsCmd = &cobra.Command{
		GroupID: "outputs",
		Use:     "runs [run-id]",
		Short:   aliasShort,
		Args:    resultsCmd.Args,
		PreRunE: resultsCmd.PreRunE,
		RunE:    resultsCmd.RunE,
		Hidden:  true,
	}
	runsCmd.Flags().AddFlagSet(resultsCmd.Flags())

	runsGetCmd := &cobra.Command{
		Use:     "get [run-id]",
		Short:   aliasShort,
		Args:    resultsCmd.Args,
		PreRunE: resultsCmd.PreRunE,
		RunE:    resultsCmd.RunE,
		Hidden:  true,
	}
	runsGetCmd.Flags().AddFlagSet(resultsCmd.Flags())

	runsShowCmd := &cobra.Command{
		Use:     "show [run-id]",
		Short:   aliasShort,
		Args:    resultsCmd.Args,
		PreRunE: resultsCmd.PreRunE,
		RunE:    resultsCmd.RunE,
		Hidden:  true,
	}
	runsShowCmd.Flags().AddFlagSet(resultsCmd.Flags())

	runsCmd.AddCommand(runsGetCmd)
	runsCmd.AddCommand(runsShowCmd)

	rootCmd.AddCommand(runsCmd)
}
