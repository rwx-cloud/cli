package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rwx-cloud/rwx/internal/skill"
	"github.com/spf13/cobra"
)

var (
	skillCmd = &cobra.Command{
		GroupID: "setup",
		Use:     "skill",
		Short:   "Agent skill related commands",
	}

	skillStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show the status of RWX agent skill installations",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := skill.Detect()
			if err != nil {
				return err
			}

			if useJsonOutput() {
				return outputSkillStatusJSON(result)
			}

			outputSkillStatusText(result)
			return nil
		},
	}
)

func init() {
	skillCmd.AddCommand(skillStatusCmd)
}

func outputSkillStatusJSON(result *skill.DetectResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result.Installations)
}

func outputSkillStatusText(result *skill.DetectResult) {
	var detected []skill.Installation
	var notDetected []skill.Installation

	for _, inst := range result.Installations {
		if skill.IsDetected(inst) {
			detected = append(detected, inst)
		} else {
			notDetected = append(notDetected, inst)
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	if len(detected) > 0 {
		fmt.Fprintln(os.Stdout, "Agent Skill Installations")
		for _, inst := range detected {
			version := inst.Version
			if version == "" {
				version = "installed"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\n", inst.Scope, shortenPath(inst.Path), version)
		}
		w.Flush()
		fmt.Fprintln(os.Stdout)
	}

	if len(notDetected) > 0 {
		fmt.Fprintln(os.Stdout, "Not detected")
		for _, inst := range notDetected {
			fmt.Fprintf(w, "  %s\t%s\n", inst.Scope, shortenPath(inst.Path))
		}
		w.Flush()
		fmt.Fprintln(os.Stdout)
	}

	fmt.Fprintln(os.Stdout, "To install:")
	fmt.Fprintln(os.Stdout, "  npx skills add rwx-cloud/skills")
}

func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
