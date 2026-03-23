package skill

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSkillVersion(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "valid frontmatter with version",
			content: `---
metadata:
  version: "1.2.0"
---
# RWX Skill
`,
			expected: "1.2.0",
		},
		{
			name:     "no frontmatter",
			content:  "# RWX Skill\nSome content",
			expected: "",
		},
		{
			name: "frontmatter without version",
			content: `---
metadata:
  name: rwx
---
# RWX Skill
`,
			expected: "",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := parseSkillVersion(tt.content)
			require.Equal(t, tt.expected, version)
		})
	}
}

func TestParseSkillVersionWithoutMetadata(t *testing.T) {
	content := `---
name: rwx
description: Some skill
---
# Content
`
	version := parseSkillVersion(content)
	require.Equal(t, "", version)
}

func TestIsDetected(t *testing.T) {
	require.True(t, IsDetected(Installation{Detected: true, Version: "1.0.0"}))
	require.True(t, IsDetected(Installation{Detected: true}))
	require.False(t, IsDetected(Installation{Detected: false}))
}
