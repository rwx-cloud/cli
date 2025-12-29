package main_test

import (
	"testing"

	rwx "github.com/rwx-cloud/cli/cmd/rwx"
	"github.com/stretchr/testify/require"
)

func TestParseInitParameters(t *testing.T) {
	t.Run("should parse init parameters", func(t *testing.T) {
		parsed, err := rwx.ParseInitParameters([]string{"a=b", "c=d"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"a": "b", "c": "d"}, parsed)
	})

	t.Run("should parse init parameter with equals signs", func(t *testing.T) {
		parsed, err := rwx.ParseInitParameters([]string{"a=b=c=d"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"a": "b=c=d"}, parsed)
	})

	t.Run("should error if init parameter is not equals-delimited", func(t *testing.T) {
		parsed, err := rwx.ParseInitParameters([]string{"a"})
		require.Nil(t, parsed)
		require.EqualError(t, err, "unable to parse \"a\"")
	})
}

func TestRunSubcommands(t *testing.T) {
	rootCmd := rwx.GetRootCmd()
	require.NotNil(t, rootCmd)

	runCmd, _, err := rootCmd.Find([]string{"run"})
	require.NoError(t, err)
	require.NotNil(t, runCmd)

	t.Run("run command should have list subcommand", func(t *testing.T) {
		listCmd, _, err := rootCmd.Find([]string{"run", "list"})
		require.NoError(t, err)
		require.NotNil(t, listCmd)
		require.Equal(t, "list", listCmd.Name())
		require.Equal(t, "List runs", listCmd.Short)
		require.Equal(t, "list [flags]", listCmd.Use)
	})

	t.Run("run command should have view subcommand", func(t *testing.T) {
		viewCmd, _, err := rootCmd.Find([]string{"run", "view"})
		require.NoError(t, err)
		require.NotNil(t, viewCmd)
		require.Equal(t, "view", viewCmd.Name())
		require.Equal(t, "View a run", viewCmd.Short)
		require.Equal(t, "view <runId> [flags]", viewCmd.Use)
	})

	t.Run("view subcommand should require exactly one argument", func(t *testing.T) {
		viewCmd, _, err := rootCmd.Find([]string{"run", "view"})
		require.NoError(t, err)
		require.NotNil(t, viewCmd)

		err = viewCmd.Args(viewCmd, []string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "accepts 1 arg(s)")

		err = viewCmd.Args(viewCmd, []string{"run-id-123"})
		require.NoError(t, err)

		err = viewCmd.Args(viewCmd, []string{"run-id-123", "extra"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "accepts 1 arg(s)")
	})
}
