package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_ResolvingBaseLayers(t *testing.T) {
	type baseLayerSetup struct {
		s            *testSetup
		apiOs        string
		apiTag       string
		apiArch      string
		apiCallCount int
		apiError     func(callCount int) error
		workingDir   string
		mintDir      string
	}

	setupBaseLayer := func(t *testing.T) *baseLayerSetup {
		s := setupTest(t)

		bl := &baseLayerSetup{
			s:            s,
			apiOs:        "gentoo 99",
			apiTag:       "1.2",
			apiArch:      "x86_64",
			apiCallCount: 0,
			apiError:     func(callCount int) error { return nil },
		}

		bl.workingDir = filepath.Join(s.tmp, "subdir1/subdir2")
		err := os.MkdirAll(bl.workingDir, 0o755)
		require.NoError(t, err)

		bl.mintDir = filepath.Join(s.tmp, "subdir1/.mint")
		err = os.MkdirAll(bl.mintDir, 0o755)
		require.NoError(t, err)

		err = os.Chdir(bl.workingDir)
		require.NoError(t, err)

		s.mockAPI.MockResolveBaseLayer = func(cfg api.ResolveBaseLayerConfig) (api.ResolveBaseLayerResult, error) {
			bl.apiCallCount += 1
			if err := bl.apiError(bl.apiCallCount); err != nil {
				return api.ResolveBaseLayerResult{}, err
			}

			os := cfg.Os
			if os == "" {
				os = bl.apiOs
			}
			tag := cfg.Tag
			if tag == "" {
				tag = bl.apiTag
			}
			arch := cfg.Arch
			if arch == "" {
				arch = bl.apiArch
			}
			return api.ResolveBaseLayerResult{
				Os:   os,
				Tag:  tag,
				Arch: arch,
			}, nil
		}

		return bl
	}

	t.Run("when no yaml files found in the default directory", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "bar.json"), []byte("some json"), 0o644)
		require.NoError(t, err)

		_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})

		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("no files provided, and no yaml files found in directory %s", bl.mintDir))
	})

	t.Run("when yaml file is actually json", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "bar.yaml"), []byte(`{
"tasks": [
  { "key": "a" },
  { "key": "b" }
]
}`), 0o644)
		require.NoError(t, err)

		_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})

		require.NoError(t, err)
		require.Equal(t, "", bl.s.mockStderr.String())
		require.Contains(t, bl.s.mockStdout.String(), "No run files were missing base")
	})

	t.Run("when yaml file doesn't include base", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "foo.txt"), []byte("some txt"), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(bl.mintDir, "bar.yaml"), []byte(`
tasks:
  - key: a
  - key: b
`), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(bl.mintDir, "baz.yaml"), []byte(`
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`), 0o644)
		require.NoError(t, err)

		t.Run("adds base to file", func(t *testing.T) {
			_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
			require.NoError(t, err)

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "bar.yaml"))
			require.NoError(t, err)
			require.Equal(t, `base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`, string(contents))

			require.Equal(t, fmt.Sprintf(
				"Added base to the following run definitions:\n%s\n",
				"\t../.mint/bar.yaml → gentoo 99, tag 1.2",
			), bl.s.mockStdout.String())

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "baz.yaml"))
			require.NoError(t, err)
			require.Equal(t, `
not-my-key:
  - key: qux
    call: mint/setup-node 1.2.3
`, string(contents))
		})

		t.Run("adds base to only a targeted file", func(t *testing.T) {
			bl.s.mockStdout.Reset()

			err := os.WriteFile(filepath.Join(bl.mintDir, "bar.yaml"), []byte(`
tasks:
  - key: a
  - key: b
`), 0o644)
			require.NoError(t, err)

			originalQuxContents := `
tasks:
  - key: a
  - key: b
`
			err = os.WriteFile(filepath.Join(bl.mintDir, "qux.yaml"), []byte(originalQuxContents), 0o644)
			require.NoError(t, err)

			_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{
				Files: []string{"../.mint/bar.yaml"},
			})
			require.NoError(t, err)

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "bar.yaml"))
			require.NoError(t, err)
			require.Equal(t, `base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`, string(contents))

			require.Equal(t, fmt.Sprintf(
				"Added base to the following run definitions:\n%s\n",
				"\t../.mint/bar.yaml → gentoo 99, tag 1.2",
			), bl.s.mockStdout.String())

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "qux.yaml"))
			require.NoError(t, err)
			require.Equal(t, originalQuxContents, string(contents))
		})

		t.Run("errors when given a file that does not exist", func(t *testing.T) {
			_, err := bl.s.service.ResolveBase(cli.ResolveBaseConfig{
				Files: []string{"does-not-exist.yaml"},
			})
			require.Error(t, err)
			require.Equal(t, "reading rwx directory entries at does-not-exist.yaml: file does not exist", err.Error())
		})
	})

	t.Run("when yaml file has a base with os but no tag or arch", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

base:
  os: gentoo 99

tasks:
  - key: a
  - key: b
`), 0o644)
		require.NoError(t, err)

		_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
		require.NoError(t, err)

		contents, err := os.ReadFile(filepath.Join(bl.mintDir, "ci.yaml"))
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push: {}

base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`, string(contents))

		require.Equal(t, fmt.Sprintf(
			"Added base to the following run definitions:\n%s\n",
			"\t../.mint/ci.yaml → gentoo 99, tag 1.2",
		), bl.s.mockStdout.String())
	})

	t.Run("when yaml file has a base with os and arch but no tag", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

base:
  os: gentoo 99
  arch: quantum

tasks:
  - key: a
  - key: b
`), 0o644)
		require.NoError(t, err)

		_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
		require.NoError(t, err)

		contents, err := os.ReadFile(filepath.Join(bl.mintDir, "ci.yaml"))
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push: {}

base:
  os: gentoo 99
  arch: quantum
  tag: 1.2

tasks:
  - key: a
  - key: b
`, string(contents))

		require.Equal(t, fmt.Sprintf(
			"Added base to the following run definitions:\n%s\n",
			"\t../.mint/ci.yaml → gentoo 99, tag 1.2",
		), bl.s.mockStdout.String())
	})

	t.Run("when yaml file has base after tasks with os but no tag", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

tasks:
  - key: a
  - key: b

base:
  os: gentoo 99`), 0o644)
		require.NoError(t, err)

		_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
		require.NoError(t, err)

		contents, err := os.ReadFile(filepath.Join(bl.mintDir, "ci.yaml"))
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push: {}

tasks:
  - key: a
  - key: b

base:
  os: gentoo 99
  tag: 1.2
`, string(contents))

		require.Equal(t, fmt.Sprintf(
			"Added base to the following run definitions:\n%s\n",
			"\t../.mint/ci.yaml → gentoo 99, tag 1.2",
		), bl.s.mockStdout.String())
	})

	t.Run("with multiple yaml files", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "one.yaml"), []byte(`tasks:
  - key: a
  - key: b
`), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(bl.mintDir, "two.yaml"), []byte(`base:
  os: gentoo 88

tasks:
  - key: c
  - key: d
`), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(bl.mintDir, "three.yaml"), []byte(`tasks:
  - key: e
  - key: f
`), 0o644)
		require.NoError(t, err)

		t.Run("updates all files", func(t *testing.T) {
			_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
			require.NoError(t, err)

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "one.yaml"))
			require.NoError(t, err)
			require.Equal(t, `base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: a
  - key: b
`, string(contents))

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "two.yaml"))
			require.NoError(t, err)
			require.Equal(t, `base:
  os: gentoo 88
  tag: 1.2

tasks:
  - key: c
  - key: d
`, string(contents))

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "three.yaml"))
			require.NoError(t, err)
			require.Equal(t, `base:
  os: gentoo 99
  tag: 1.2

tasks:
  - key: e
  - key: f
`, string(contents))

			require.Equal(t, fmt.Sprintf(
				"Added base to the following run definitions:\n%s\n%s\n%s\n",
				"\t../.mint/one.yaml → gentoo 99, tag 1.2",
				"\t../.mint/three.yaml → gentoo 99, tag 1.2",
				"\t../.mint/two.yaml → gentoo 88, tag 1.2",
			), bl.s.mockStdout.String())
		})

		t.Run("when an API request fails", func(t *testing.T) {
			bl.apiCallCount = 0

			err := os.WriteFile(filepath.Join(bl.mintDir, "one.yaml"), []byte(`tasks:
  - key: a
  - key: b
`), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(bl.mintDir, "two.yaml"), []byte(`base:
  os: gentoo 88

tasks:
  - key: c
  - key: d
`), 0o644)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(bl.mintDir, "three.yaml"), []byte(`tasks:
  - key: e
  - key: f
`), 0o644)
			require.NoError(t, err)

			contentsOne, err := os.ReadFile(filepath.Join(bl.mintDir, "one.yaml"))
			require.NoError(t, err)
			contentsTwo, err := os.ReadFile(filepath.Join(bl.mintDir, "two.yaml"))
			require.NoError(t, err)
			contentsThree, err := os.ReadFile(filepath.Join(bl.mintDir, "three.yaml"))
			require.NoError(t, err)

			bl.apiError = func(callCount int) error {
				if callCount == 2 {
					return errors.New("API request failed")
				}
				return nil
			}

			_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
			require.Error(t, err)
			require.Contains(t, err.Error(), "API request failed")

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "one.yaml"))
			require.NoError(t, err)
			require.Equal(t, string(contentsOne), string(contents))

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "two.yaml"))
			require.NoError(t, err)
			require.Equal(t, string(contentsTwo), string(contents))

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "three.yaml"))
			require.NoError(t, err)
			require.Equal(t, string(contentsThree), string(contents))
		})
	})

	t.Run("when yaml file with only embedded runs doesn't include base", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "foo.yaml"), []byte(`
tasks:
  - key: a
    call: ${{ run.dir }}/bar.yaml
`), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(bl.mintDir, "bar.yaml"), []byte(`
tasks:
  - key: b
    run: /bin/true
`), 0o644)
		require.NoError(t, err)

		t.Run("does not add base to file", func(t *testing.T) {
			_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
			require.NoError(t, err)

			var contents []byte

			contents, err = os.ReadFile(filepath.Join(bl.mintDir, "foo.yaml"))
			require.NoError(t, err)
			require.Equal(t, `
tasks:
  - key: a
    call: ${{ run.dir }}/bar.yaml
`, string(contents))
		})
	})

	t.Run("when yaml file has a custom base image it does not add os/tag", func(t *testing.T) {
		bl := setupBaseLayer(t)

		err := os.WriteFile(filepath.Join(bl.mintDir, "ci.yaml"), []byte(`on:
  github:
    push: {}

base:
	image: alpine:latest
	config: none

tasks:
  - key: a
  - key: b
`), 0o644)
		require.NoError(t, err)

		_, err = bl.s.service.ResolveBase(cli.ResolveBaseConfig{})
		require.NoError(t, err)

		contents, err := os.ReadFile(filepath.Join(bl.mintDir, "ci.yaml"))
		require.NoError(t, err)
		require.Equal(t, `on:
  github:
    push: {}

base:
	image: alpine:latest
	config: none

tasks:
  - key: a
  - key: b
`, string(contents))

		require.Equal(t, "No run files were missing base.\n", bl.s.mockStdout.String())
	})
}
