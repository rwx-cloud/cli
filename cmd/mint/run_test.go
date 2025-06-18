package main_test

import (
	"testing"

	mint "github.com/rwx-research/mint-cli/cmd/mint"
	"github.com/stretchr/testify/require"
)

func TestParseInitParameters(t *testing.T) {
	t.Run("should parse init parameters", func(t *testing.T) {
		parsed, err := mint.ParseInitParameters([]string{"a=b", "c=d"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"a": "b", "c": "d"}, parsed)
	})

	t.Run("should parse init parameter with equals signs", func(t *testing.T) {
		parsed, err := mint.ParseInitParameters([]string{"a=b=c=d"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"a": "b=c=d"}, parsed)
	})

	t.Run("should error if init parameter is not equals-delimited", func(t *testing.T) {
		parsed, err := mint.ParseInitParameters([]string{"a"})
		require.Nil(t, parsed)
		require.EqualError(t, err, "unable to parse \"a\"")
	})
}
