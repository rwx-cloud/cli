package cli_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/rwx-research/mint-cli/internal/errors"
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
