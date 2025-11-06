package cli_test

import (
	"fmt"
	"testing"

	"github.com/distribution/reference"
	"github.com/docker/cli/cli/config/types"
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

type mockDockerAuthConfigurator struct {
	config types.AuthConfig
	err    error
}

func (m mockDockerAuthConfigurator) GetAuthConfig(host string) (types.AuthConfig, error) {
	if m.err != nil {
		return types.AuthConfig{}, fmt.Errorf("failed to get auth config for host %q: %w", host, m.err)
	}

	return m.config, nil
}

func ReferenceMustParse(t *testing.T, ref string) reference.Named {
	parsed, err := reference.ParseNormalizedNamed(ref)
	require.NoError(t, err)
	return parsed
}

func TestService_PushImage(t *testing.T) {
	t.Run("only supports one registry", func(t *testing.T) {
		s := setupTest(t)

		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "a.registry.com/repo-one:tag-one"),
				ReferenceMustParse(t, "b.registry.com/repo-one:tag-two"),
				ReferenceMustParse(t, "c.registry.com/repo-one:tag-three"),
			},
			DockerCLI: mockDockerAuthConfigurator{},
			JSON:      false,
			Wait:      true,
			OpenURL:   func(url string) error { return nil },
		}

		err := s.service.PushImage(cfg)

		require.Error(t, err)
		require.Equal(t, "all image references must have the same registry: a.registry.com != b.registry.com", err.Error())
	})

	t.Run("only supports one repository", func(t *testing.T) {
		s := setupTest(t)

		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo-one:tag-one"),
				ReferenceMustParse(t, "registry.com/repo-two:tag-two"),
				ReferenceMustParse(t, "registry.com/repo-three:tag-three"),
			},
			DockerCLI: mockDockerAuthConfigurator{},
			JSON:      false,
			Wait:      true,
			OpenURL:   func(url string) error { return nil },
		}

		err := s.service.PushImage(cfg)

		require.Error(t, err)
		require.Equal(t, "all image references must have the same repository: repo-one != repo-two", err.Error())
	})

	t.Run("supports dockerhub", func(t *testing.T) {
		s := setupTest(t)

		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "postgres:latest"),
				ReferenceMustParse(t, "postgres:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{err: fmt.Errorf("no credentials available")},
			JSON:      false,
			Wait:      true,
			OpenURL:   func(url string) error { return nil },
		}

		err := s.service.PushImage(cfg)

		require.Error(t, err)
		require.Equal(
			t,
			"unable to get credentials for registry \"registry.hub.docker.com/v2\" from docker: "+
				"failed to get auth config for host \"index.docker.io\": "+
				"no credentials available",
			err.Error(),
		)
	})

	t.Run("support other registries", func(t *testing.T) {
		s := setupTest(t)

		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{err: fmt.Errorf("no credentials available")},
			JSON:      false,
			Wait:      true,
			OpenURL:   func(url string) error { return nil },
		}

		err := s.service.PushImage(cfg)

		require.Error(t, err)
		require.Equal(
			t,
			"unable to get credentials for registry \"registry.com\" from docker: "+
				"failed to get auth config for host \"registry.com\": "+
				"no credentials available",
			err.Error(),
		)
	})

	t.Run("fails when starting the push fails", func(t *testing.T) {
		s := setupTest(t)
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)

			return api.StartImagePushResult{}, fmt.Errorf("failed to start push")
		}

		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{config: types.AuthConfig{Username: "my-username", Password: "my-password"}},
			JSON:      false,
			Wait:      true,
			OpenURL: func(url string) error {
				require.Fail(t, "open url should not be called when the push does not start")
				return nil
			},
		}

		err := s.service.PushImage(cfg)

		require.Error(t, err)
		require.Equal(t, "failed to start push", err.Error())

		require.Empty(t, s.mockStdout.String())
		require.Contains(t, s.mockStderr.String(), "Starting image push of task \"some-task-id\" to 'registry.com/repo' with tags: latest, 17.1...")
	})

	t.Run("when the push status errors", func(t *testing.T) {
		s := setupTest(t)
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{}, fmt.Errorf("failed to get push status")
		}

		didOpenURL := false
		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{config: types.AuthConfig{Username: "my-username", Password: "my-password"}},
			JSON:      false,
			Wait:      true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		err := s.service.PushImage(cfg)

		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to get image push status")

		require.True(t, didOpenURL)

		require.Empty(t, s.mockStdout.String())
		require.Contains(t, s.mockStderr.String(), "Starting image push of task \"some-task-id\" to 'registry.com/repo' with tags: latest, 17.1...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("when the push ultimately fails", func(t *testing.T) {
		s := setupTest(t)
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "failed"}, nil
		}

		didOpenURL := false
		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{config: types.AuthConfig{Username: "my-username", Password: "my-password"}},
			JSON:      false,
			Wait:      true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		err := s.service.PushImage(cfg)

		require.Error(t, err)
		require.Equal(t, "image push failed, inspect the run at \"some-run-url\" to see why", err.Error())

		require.True(t, didOpenURL)

		require.Empty(t, s.mockStdout.String())
		require.Contains(t, s.mockStderr.String(), "Starting image push of task \"some-task-id\" to 'registry.com/repo' with tags: latest, 17.1...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("when the push ultimately succeeds", func(t *testing.T) {
		s := setupTest(t)
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "succeeded"}, nil
		}

		didOpenURL := false
		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{config: types.AuthConfig{Username: "my-username", Password: "my-password"}},
			JSON:      false,
			Wait:      true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		err := s.service.PushImage(cfg)

		require.NoError(t, err)

		require.True(t, didOpenURL)

		require.Contains(t, s.mockStdout.String(), "Image push succeeded! You can pull your image")
		require.Contains(t, s.mockStderr.String(), "Starting image push of task \"some-task-id\" to 'registry.com/repo' with tags: latest, 17.1...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("prefers environment variable credentials over docker credentials", func(t *testing.T) {
		s := setupTest(t)

		t.Setenv("RWX_PUSH_USERNAME", "env-username")
		t.Setenv("RWX_PUSH_PASSWORD", "env-password")

		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "env-username", cfg.Credentials.Username)
			require.Equal(t, "env-password", cfg.Credentials.Password)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "succeeded"}, nil
		}

		didOpenURL := false
		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{config: types.AuthConfig{Username: "my-username", Password: "my-password"}},
			JSON:      false,
			Wait:      true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		err := s.service.PushImage(cfg)

		require.NoError(t, err)

		require.True(t, didOpenURL)

		require.Contains(t, s.mockStdout.String(), "Image push succeeded! You can pull your image")
		require.Contains(t, s.mockStderr.String(), "Starting image push of task \"some-task-id\" to 'registry.com/repo' with tags: latest, 17.1...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("errors if only RWX_PUSH_USERNAME is set", func(t *testing.T) {
		s := setupTest(t)

		t.Setenv("RWX_PUSH_USERNAME", "env-username")

		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "env-username", cfg.Credentials.Username)
			require.Equal(t, "env-password", cfg.Credentials.Password)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "succeeded"}, nil
		}

		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{config: types.AuthConfig{Username: "my-username", Password: "my-password"}},
			JSON:      false,
			Wait:      true,
			OpenURL: func(url string) error {
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		err := s.service.PushImage(cfg)

		require.ErrorContains(t, err, "RWX_PUSH_PASSWORD must be set if RWX_PUSH_USERNAME is set")
	})

	t.Run("errors if only RWX_PUSH_PASSWORD is set", func(t *testing.T) {
		s := setupTest(t)

		t.Setenv("RWX_PUSH_PASSWORD", "env-password")

		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "env-username", cfg.Credentials.Username)
			require.Equal(t, "env-password", cfg.Credentials.Password)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "succeeded"}, nil
		}

		cfg := cli.PushImageConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			DockerCLI: mockDockerAuthConfigurator{config: types.AuthConfig{Username: "my-username", Password: "my-password"}},
			JSON:      false,
			Wait:      true,
			OpenURL: func(url string) error {
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		err := s.service.PushImage(cfg)

		require.ErrorContains(t, err, "RWX_PUSH_USERNAME must be set if RWX_PUSH_PASSWORD is set")
	})
}
