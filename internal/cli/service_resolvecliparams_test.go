package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveCliParams(t *testing.T) {
	t.Run("errors when no on section exists", func(t *testing.T) {
		input := `
base:
  os: ubuntu 24.04
  tag: 1.2

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err := ResolveCliParams(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no git init params found")
	})

	t.Run("errors when on section is empty", func(t *testing.T) {
		input := `
on:

base:
  os: ubuntu 24.04
  tag: 1.2

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err := ResolveCliParams(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no git init params found")
	})

	t.Run("errors when triggers only have non-git init params", func(t *testing.T) {
		input := `
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
		_, err := ResolveCliParams(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no git init params found")
	})

	t.Run("no changes when CLI trigger already has git init params", func(t *testing.T) {
		input := `
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
		output, err := ResolveCliParams(input)
		require.NoError(t, err)
		require.Equal(t, input, output)
	})

	t.Run("adds CLI trigger when another trigger has git params", func(t *testing.T) {
		input := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		expected := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}
  cli:
    init:
      sha: ${{ event.git.sha }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		output, err := ResolveCliParams(input)
		require.NoError(t, err)
		require.Equal(t, expected, output)
	})

	t.Run("merges git params into existing CLI trigger", func(t *testing.T) {
		input := `
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
		expected := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}
  cli:
    init:
      foo: bar
      sha: ${{ event.git.sha }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		output, err := ResolveCliParams(input)
		require.NoError(t, err)
		require.Equal(t, expected, output)
	})

	t.Run("errors on conflicting git param mappings", func(t *testing.T) {
		input := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}
  gitlab:
    push:
      init:
        sha: ${{ event.git.commit }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		_, err := ResolveCliParams(input)
		require.Error(t, err)
		require.Contains(t, err.Error(), "conflict")
	})

	t.Run("succeeds when multiple triggers have same git param mappings", func(t *testing.T) {
		input := `
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
		expected := `
on:
  github:
    push:
      init:
        sha: ${{ event.git.sha }}
  gitlab:
    push:
      init:
        sha: ${{ event.git.sha }}
  cli:
    init:
      sha: ${{ event.git.sha }}

tasks:
  - key: "test"
    run: echo 'hello world'
`
		output, err := ResolveCliParams(input)
		require.NoError(t, err)
		require.Equal(t, expected, output)
	})
}
