package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func newTestCommand(flags map[string]string, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "test",
		Args: cobra.ArbitraryArgs,
		Run:  func(cmd *cobra.Command, args []string) {},
	}

	for name, defaultVal := range flags {
		cmd.Flags().String(name, defaultVal, "")
	}

	var cliArgs []string
	for name, val := range flags {
		cliArgs = append(cliArgs, "--"+name, val)
	}
	cliArgs = append(cliArgs, args...)

	// Execute to parse flags and populate Args()
	cmd.SetArgs(cliArgs)
	_ = cmd.Execute()

	return cmd
}

func TestSafeTelemetryProps(t *testing.T) {
	t.Run("returns nil for unlisted command", func(t *testing.T) {
		cmd := newTestCommand(nil, nil)
		props := safeTelemetryProps("login", cmd)
		require.Nil(t, props)
	})

	t.Run("returns nil when cmd is nil", func(t *testing.T) {
		props := safeTelemetryProps("run", nil)
		require.Nil(t, props)
	})

	t.Run("includes only safe flag values", func(t *testing.T) {
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		cmd.Flags().String("dir", "", "")
		cmd.Flags().String("access-token", "", "")
		cmd.Flags().String("target", "", "")

		cmd.SetArgs([]string{"--dir", "/tmp", "--access-token", "secret", "--target", "build"})
		_ = cmd.Execute()

		props := safeTelemetryProps("run", cmd)
		require.NotNil(t, props)

		flagValues := props["flag_values"].(map[string]string)
		require.Equal(t, "/tmp", flagValues["dir"])
		require.Equal(t, "build", flagValues["target"])
		require.NotContains(t, flagValues, "access-token")
	})

	t.Run("omits flag_values when no safe flags are set", func(t *testing.T) {
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		cmd.Flags().String("access-token", "", "")

		cmd.SetArgs([]string{"--access-token", "secret"})
		_ = cmd.Execute()

		props := safeTelemetryProps("run", cmd)
		require.NotNil(t, props)
		require.NotContains(t, props, "flag_values")
	})

	t.Run("includes positional args when allowed", func(t *testing.T) {
		cmd := newTestCommand(nil, []string{"file1.yml", "file2.yml"})

		props := safeTelemetryProps("lint", cmd)
		require.NotNil(t, props)

		require.Equal(t, []string{"file1.yml", "file2.yml"}, props["args"])
	})

	t.Run("excludes positional args when not allowed", func(t *testing.T) {
		cmd := newTestCommand(nil, []string{"echo", "hello"})

		props := safeTelemetryProps("sandbox exec", cmd)
		require.NotNil(t, props)
		require.NotContains(t, props, "args")
	})

	t.Run("empty props when command listed but nothing set", func(t *testing.T) {
		cmd := newTestCommand(nil, nil)

		props := safeTelemetryProps("sandbox list", cmd)
		require.NotNil(t, props)
		require.Empty(t, props)
	})

	t.Run("includes both flag values and args", func(t *testing.T) {
		cmd := &cobra.Command{
			Use:  "test",
			Args: cobra.ArbitraryArgs,
			Run:  func(cmd *cobra.Command, args []string) {},
		}
		cmd.Flags().String("timeout", "", "")
		cmd.Flags().String("dir", "", "")

		cmd.SetArgs([]string{"--timeout", "10s", "--dir", "/app", "config.yml"})
		_ = cmd.Execute()

		props := safeTelemetryProps("lint", cmd)
		require.NotNil(t, props)

		flagValues := props["flag_values"].(map[string]string)
		require.Equal(t, "10s", flagValues["timeout"])
		require.Equal(t, "/app", flagValues["dir"])
		require.Equal(t, []string{"config.yml"}, props["args"])
	})
}
