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
