package cli_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/rwx-cloud/cli/internal/errors"
	"github.com/stretchr/testify/require"
)

func TestYamlDoc_TryReadStringAtPath(t *testing.T) {
	t.Run("returns an empty string when the path is not found", func(t *testing.T) {
		contents := `
a:
  b: hello
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.Equal(t, "", doc.TryReadStringAtPath("$.a.c"))
	})

	t.Run("returns a string value at the given path", func(t *testing.T) {
		contents := `
a:
  b: hello
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		value, err := doc.ReadStringAtPath("$.a.b")
		require.NoError(t, err)
		require.Equal(t, "hello", value)
	})

	t.Run("returns false when tasks are not found", func(t *testing.T) {
		contents := `
tasks-but-not-really:
  - key: task1
    tasks: [still, no]
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.False(t, doc.HasTasks())
	})
}

func TestYamlDoc_HasTasks(t *testing.T) {
	t.Run("returns true when tasks are found", func(t *testing.T) {
		contents := `
on:
  github:

tasks:
  - key: task1
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.True(t, doc.HasTasks())
	})

	t.Run("returns false when tasks are not found", func(t *testing.T) {
		contents := `
tasks-but-not-really:
  - key: task1
    tasks: [still, no]
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.False(t, doc.HasTasks())
	})
}

func TestYamlDoc_InsertBefore(t *testing.T) {
	t.Run("inserts a yaml object before the given path", func(t *testing.T) {
		contents := `
on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

tasks:
  - key: task1 # another line comment
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.InsertBefore("$.tasks", map[string]any{
			"base": map[string]any{
				"os":   "linux",
				"tag":  1.2,
				"arch": "x86_64",
			},
		})
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

base:
  arch: x86_64
  os: linux
  tag: 1.2

tasks:
  - key: task1 # another line comment
  - key: task2
`, doc.String())
	})

	t.Run("errors when the path is not found", func(t *testing.T) {
		contents := `
tasks:
  - key: task1
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.MergeAtPath("$.base", map[string]any{
			"tag":  1.2,
			"arch": "x86_64",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find path ( $.base ): node not found")
		require.True(t, errors.Is(err, yaml.ErrNotFoundNode))
	})

	t.Run("inserts before leading comments", func(t *testing.T) {
		contents := `# This is a multiline
# comment
tasks:
  - key: a
    run: echo a`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.InsertBefore("$.tasks", map[string]any{
			"base": map[string]any{
				"os":  "ubuntu 24.04",
				"tag": 1.1,
			},
		})
		require.NoError(t, err)
		require.Equal(t, `base:
  os: ubuntu 24.04
  tag: 1.1

# This is a multiline
# comment
tasks:
  - key: a
    run: echo a
`, doc.String())
	})

	t.Run("inserts after single line comment", func(t *testing.T) {
		contents := `# Single comment
tasks:
  - key: a
    run: echo a`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.InsertBefore("$.tasks", map[string]any{
			"base": map[string]any{
				"os":  "ubuntu 24.04",
				"tag": 1.1,
			},
		})
		require.NoError(t, err)
		require.Equal(t, `base:
  os: ubuntu 24.04
  tag: 1.1

# Single comment
tasks:
  - key: a
    run: echo a
`, doc.String())
	})

	t.Run("preserves comment block structure", func(t *testing.T) {
		contents := `# Header comment
# Continued comment

# Another comment block
tasks:
  - key: a
    run: echo a`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.InsertBefore("$.tasks", map[string]any{
			"base": map[string]any{
				"os":  "ubuntu 24.04",
				"tag": 1.1,
			},
		})
		require.NoError(t, err)
		require.Equal(t, `# Header comment
# Continued comment

base:
  os: ubuntu 24.04
  tag: 1.1

# Another comment block
tasks:
  - key: a
    run: echo a
`, doc.String())
	})

	t.Run("handles mixed content and comments", func(t *testing.T) {
		contents := `# File header
config:
  value: true

# Task comments
tasks:
  - key: a
    run: echo a`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.InsertBefore("$.tasks", map[string]any{
			"base": map[string]any{
				"os":  "ubuntu 24.04",
				"tag": 1.1,
			},
		})
		require.NoError(t, err)
		require.Equal(t, `# File header
config:
  value: true

base:
  os: ubuntu 24.04
  tag: 1.1

# Task comments
tasks:
  - key: a
    run: echo a
`, doc.String())
	})
}

func TestYamlDoc_MergeAtPath(t *testing.T) {
	t.Run("merges a yaml object at a specific path", func(t *testing.T) {
		contents := `
on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

base:
  # comment
  os: linux

tasks:
  - key: task1 # another line comment
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.MergeAtPath("$.base", map[string]any{
			"tag":  1.2,
			"arch": "x86_64",
		})
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

base:
  # comment
  os: linux
  arch: x86_64
  tag: 1.2

tasks:
  - key: task1 # another line comment
  - key: task2
`, doc.String())
	})

	t.Run("errors when the path is not found", func(t *testing.T) {
		contents := `
tasks:
  - key: task1
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.MergeAtPath("$.base", map[string]any{
			"tag":  1.2,
			"arch": "x86_64",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find path ( $.base ): node not found")
		require.True(t, errors.Is(err, yaml.ErrNotFoundNode))
	})
}

func TestYamlDoc_ReplaceAtPath(t *testing.T) {
	t.Run("replaces a yaml file at a specific path", func(t *testing.T) {
		contents := `
on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

base:
  # comment
  os: linux
  tag: 1.0   # comment here
  arch: x86_64

tasks:
  - key: task1 # another line comment
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.ReplaceAtPath("$.base.tag", 1.2)
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

base:
  # comment
  os: linux
  tag: 1.2
  arch: x86_64

tasks:
  - key: task1 # another line comment
  - key: task2
`, doc.String())
	})

	t.Run("errors when the path is not found", func(t *testing.T) {
		contents := `
tasks:
  - key: task1
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.ReplaceAtPath("$.base.tag", 1.2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find path ( $.base.tag ): node not found")
		require.True(t, errors.Is(err, yaml.ErrNotFoundNode))
	})
}

func TestYamlDoc_IsRunDefinition(t *testing.T) {
	t.Run("returns false for empty document", func(t *testing.T) {
		doc, err := cli.ParseYAMLDoc("")
		require.NoError(t, err)

		require.False(t, doc.IsRunDefinition())
	})

	t.Run("returns true for run definition with tasks", func(t *testing.T) {
		contents := `
tasks:
  - key: task1
  - key: task2
`
		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.True(t, doc.IsRunDefinition())
	})

	t.Run("returns false for list of tasks", func(t *testing.T) {
		contents := `
- key: task1
- key: task2
`
		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.False(t, doc.IsRunDefinition())
	})
}

func TestYamlDoc_IsListOfTasks(t *testing.T) {
	t.Run("returns false for empty document", func(t *testing.T) {
		doc, err := cli.ParseYAMLDoc("")
		require.NoError(t, err)

		require.False(t, doc.IsListOfTasks())
	})

	t.Run("returns true for list of tasks", func(t *testing.T) {
		contents := `
- key: task1
- key: task2
`
		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.True(t, doc.IsListOfTasks())
	})

	t.Run("returns false for run definition", func(t *testing.T) {
		contents := `
tasks:
  - key: task1
  - key: task2
`
		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		require.False(t, doc.IsListOfTasks())
	})
}

func TestYamlDoc_SetAtPath(t *testing.T) {
	t.Run("sets and overwrites a yaml object at a specific path", func(t *testing.T) {
		contents := `
on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

base:
  # comment
  old: true

tasks:
  - key: task1 # another line comment
  - key: task2
`

		doc, err := cli.ParseYAMLDoc(contents)
		require.NoError(t, err)

		err = doc.SetAtPath("$.base", map[string]any{
			"os":  "linux",
			"tag": 1.2,
		})
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}

tag: not it

base:
  os: linux
  tag: 1.2

tasks:
  - key: task1 # another line comment
  - key: task2
`, doc.String())
	})
}
