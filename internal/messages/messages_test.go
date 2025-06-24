package messages_test

import (
	"testing"

	"github.com/rwx-cloud/cli/internal/messages"
	"github.com/stretchr/testify/require"
)

func TestFormatUserMessage(t *testing.T) {
	t.Run("builds a string based on the available data", func(t *testing.T) {
		require.Equal(t, "message", messages.FormatUserMessage("message", "", []messages.StackEntry{}, ""))
		require.Equal(t, "message\nframe", messages.FormatUserMessage("message", "frame", []messages.StackEntry{}, ""))
		require.Equal(t, "message\nframe\nadvice", messages.FormatUserMessage("message", "frame", []messages.StackEntry{}, "advice"))

		stackTrace := []messages.StackEntry{
			{
				FileName: "mint1.yml",
				Line:     22,
				Column:   11,
				Name:     "*alias",
			},
			{
				FileName: "mint1.yml",
				Line:     5,
				Column:   22,
			},
		}
		require.Equal(t, `message
frame
  at mint1.yml:5:22
  at *alias (mint1.yml:22:11)
advice`, messages.FormatUserMessage("message", "frame", stackTrace, "advice"))
	})
}
