package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveCliParamsForFile(t *testing.T) {
	t.Run("no changes when no on section exists", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
base:
  os: ubuntu 24.04
  tag: 1.2

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.False(t, modified)
	})

	t.Run("no changes when on section is empty", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:

base:
  os: ubuntu 24.04
  tag: 1.2

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.False(t, modified)
	})

	t.Run("no changes when triggers only have non-git init params", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        foo: bar
        baz: qux

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.False(t, modified)
	})

	t.Run("no changes when CLI trigger already has git init params", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  cli:
    init:
      sha: ${{ event.git.sha }}
  github:
    push:
      init:
        sha: ${{ event.git.sha }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.False(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Equal(t, content, string(result))
	})

	t.Run("no changes when CLI already has git params", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  cli:
    init:
      sha: ${{ event.git.sha }}
  github:
    push:
      init:
        ref: ${{ event.git.ref }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.False(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Equal(t, content, string(result))
	})

	t.Run("adds CLI trigger when another trigger has git params", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "cli:")
		require.Contains(t, string(result), "sha: ${{ event.git.sha }}")
	})

	t.Run("merges git params into existing CLI trigger", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}
  cli:
    init:
      foo: bar

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "foo: bar")
		require.Contains(t, string(result), "sha: ${{ event.git.sha }}")
	})

	t.Run("succeeds when multiple triggers have same git param mappings", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}
  gitlab:
    push:
      init:
        sha: ${{ event.git.sha }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "cli:")
		require.Contains(t, string(result), "sha: ${{ event.git.sha }}")
	})

	t.Run("detects git/clone package and maps ref to event.git.sha", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        # git/clone ref takes precedence over this mapping
        commit-sha: ${{ event.git.sha }}

tasks:
  - key: clone
    call: git/clone 1.8.1
    with:
      ref: ${{ init.ref }}
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "cli:")
		require.Contains(t, string(result), "ref: ${{ event.git.sha }}")
	})

	t.Run("errors when multiple git/clone packages use different ref params", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}

tasks:
  - key: clone1
    call: git/clone 1.8.1
    with:
      ref: ${{ init.ref }}
  - key: clone2
    call: git/clone 1.8.1
    with:
      ref: ${{ init.sha }}
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.Error(t, err)
		require.False(t, modified)
		require.Contains(t, err.Error(), "multiple git/clone")
	})

	t.Run("uses init expression when one git/clone has hardcoded ref", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}

tasks:
  - key: clone1
    call: git/clone 1.8.1
    with:
      ref: main
  - key: clone2
    call: git/clone 1.8.1
    with:
      ref: ${{ init.ref }}
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "cli:")
		require.Contains(t, string(result), "ref: ${{ event.git.sha }}")
	})

	t.Run("adds CLI trigger when git/clone exists but no on section", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
base:
  os: ubuntu 24.04
  tag: 1.2

tasks:
  - key: clone
    call: git/clone 1.8.1
    with:
      ref: ${{ init.sha }}
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "on:")
		require.Contains(t, string(result), "cli:")
		require.Contains(t, string(result), "sha: ${{ event.git.sha }}")
	})

	t.Run("adds CLI trigger when dispatch trigger has git params", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  dispatch:
    - key: release-cli
      title: "Release"
      init:
        commit: ${{ event.git.sha }}
        version: ${{ event.dispatch.params.version }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "cli:")
		require.Contains(t, string(result), "commit: ${{ event.git.sha }}")
	})

	t.Run("always maps to event.git.sha", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
		require.NoError(t, err)
		defer tmpFile.Close()

		content := `
on:
  github:
    push:
      init:
        commit-sha: ${{ event.git.sha }}
        ref: ${{ event.git.ref }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)

		modified, err := ResolveCliParamsForFile(tmpFile.Name())
		require.NoError(t, err)
		require.True(t, modified)

		result, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		require.Contains(t, string(result), "cli:")
		require.Contains(t, string(result), "commit-sha: ${{ event.git.sha }}")
		require.Contains(t, string(result), "ref: ${{ event.git.sha }}")
	})
}
