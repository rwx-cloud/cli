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

func ReferenceMustParse(t *testing.T, ref string) reference.Named {
	parsed, err := reference.ParseNormalizedNamed(ref)
	require.NoError(t, err)
	return parsed
}

func TestService_ImagePush(t *testing.T) {
	t.Run("only supports one registry", func(t *testing.T) {
		s := setupTest(t)

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "a.registry.com/repo-one:tag-one"),
				ReferenceMustParse(t, "b.registry.com/repo-one:tag-two"),
				ReferenceMustParse(t, "c.registry.com/repo-one:tag-three"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL:     func(url string) error { return nil },
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.Error(t, err)
		require.Equal(t, "all image references must have the same registry: a.registry.com != b.registry.com", err.Error())
	})

	t.Run("only supports one repository", func(t *testing.T) {
		s := setupTest(t)

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo-one:tag-one"),
				ReferenceMustParse(t, "registry.com/repo-two:tag-two"),
				ReferenceMustParse(t, "registry.com/repo-three:tag-three"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL:     func(url string) error { return nil },
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.Error(t, err)
		require.Equal(t, "all image references must have the same repository: repo-one != repo-two", err.Error())
	})

	t.Run("supports dockerhub", func(t *testing.T) {
		s := setupTest(t)
		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{}, fmt.Errorf("failed to get auth config for host %q: no credentials available", host)
		}

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "postgres:latest"),
				ReferenceMustParse(t, "postgres:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL:     func(url string) error { return nil },
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.Error(t, err)
		require.Equal(
			t,
			"unable to get credentials for registry \"registry-1.docker.io\" from docker: "+
				"failed to get auth config for host \"index.docker.io\": "+
				"no credentials available",
			err.Error(),
		)
	})

	t.Run("support other registries", func(t *testing.T) {
		s := setupTest(t)
		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{}, fmt.Errorf("failed to get auth config for host %q: no credentials available", host)
		}

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL:     func(url string) error { return nil },
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.Error(t, err)
		require.Equal(
			t,
			"unable to get credentials for registry \"registry.com\" from docker: "+
				"failed to get auth config for host \"registry.com\": "+
				"no credentials available",
			err.Error(),
		)
	})

	t.Run("fails when the task status is ultimately failed", func(t *testing.T) {
		s := setupTest(t)
		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}

		count := 0
		s.mockAPI.MockTaskIDStatus = func(cfg api.TaskIDStatusConfig) (api.TaskStatusResult, error) {
			count++
			require.Equal(t, "some-task-id", cfg.TaskID)

			if count <= 1 {
				backoff := 10
				return api.TaskStatusResult{Status: &api.TaskStatus{Result: "pending"}, Polling: api.PollingResult{Completed: false, BackoffMs: &backoff}}, nil
			} else {
				return api.TaskStatusResult{Status: &api.TaskStatus{Result: "failed"}, Polling: api.PollingResult{Completed: true}}, nil
			}
		}

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				require.Fail(t, "open url should not be called when the push does not start")
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.Error(t, err)
		require.Equal(t, "task failed", err.Error())

		require.Contains(t, s.mockStdout.String(), "Pushing image from task: some-task-id\nregistry.com/repo:latest\nregistry.com/repo:17.1")
		require.Contains(t, s.mockStderr.String(), "Starting...")
	})

	t.Run("fails when starting the push fails", func(t *testing.T) {
		s := setupTest(t)
		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}
		s.mockAPI.MockTaskIDStatus = func(cfg api.TaskIDStatusConfig) (api.TaskStatusResult, error) {
			return api.TaskStatusResult{Status: &api.TaskStatus{Result: "succeeded"}, Polling: api.PollingResult{Completed: true}}, nil
		}
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)
			require.Equal(t, "zstd", cfg.Compression)

			return api.StartImagePushResult{}, fmt.Errorf("failed to start push")
		}

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				require.Fail(t, "open url should not be called when the push does not start")
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.Error(t, err)
		require.Equal(t, "failed to start push", err.Error())

		require.Contains(t, s.mockStdout.String(), "Pushing image from task: some-task-id\nregistry.com/repo:latest\nregistry.com/repo:17.1")
		require.Contains(t, s.mockStderr.String(), "Starting...")
	})

	t.Run("when the push status errors", func(t *testing.T) {
		s := setupTest(t)
		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}
		s.mockAPI.MockTaskIDStatus = func(cfg api.TaskIDStatusConfig) (api.TaskStatusResult, error) {
			return api.TaskStatusResult{Status: &api.TaskStatus{Result: "succeeded"}, Polling: api.PollingResult{Completed: true}}, nil
		}
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)
			require.Equal(t, "gzip", cfg.Compression)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{}, fmt.Errorf("failed to get push status")
		}

		didOpenURL := false
		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "gzip",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unable to get image push status")

		require.True(t, didOpenURL)

		require.Contains(t, s.mockStdout.String(), "Pushing image from task: some-task-id\nregistry.com/repo:latest\nregistry.com/repo:17.1\n")
		require.Contains(t, s.mockStderr.String(), "Starting...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("when the push ultimately fails", func(t *testing.T) {
		s := setupTest(t)
		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}
		s.mockAPI.MockTaskIDStatus = func(cfg api.TaskIDStatusConfig) (api.TaskStatusResult, error) {
			return api.TaskStatusResult{Status: &api.TaskStatus{Result: "succeeded"}, Polling: api.PollingResult{Completed: true}}, nil
		}
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)
			require.Equal(t, "none", cfg.Compression)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "failed"}, nil
		}

		didOpenURL := false
		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "none",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.Error(t, err)
		require.Equal(t, "image push failed, inspect the run at \"some-run-url\" to see why", err.Error())
		require.NotNil(t, result)
		require.Equal(t, "some-push-id", result.PushID)
		require.Equal(t, "some-run-url", result.RunURL)
		require.Equal(t, "failed", result.Status)

		require.True(t, didOpenURL)

		require.Contains(t, s.mockStdout.String(), "Pushing image from task: some-task-id\nregistry.com/repo:latest\nregistry.com/repo:17.1\n")
		require.Contains(t, s.mockStderr.String(), "Starting...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("when the push ultimately succeeds", func(t *testing.T) {
		s := setupTest(t)
		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}
		s.mockAPI.MockTaskIDStatus = func(cfg api.TaskIDStatusConfig) (api.TaskStatusResult, error) {
			return api.TaskStatusResult{Status: &api.TaskStatus{Result: "succeeded"}, Polling: api.PollingResult{Completed: true}}, nil
		}
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "my-username", cfg.Credentials.Username)
			require.Equal(t, "my-password", cfg.Credentials.Password)
			require.Equal(t, "zstd", cfg.Compression)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "succeeded"}, nil
		}

		didOpenURL := false
		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "some-push-id", result.PushID)
		require.Equal(t, "some-run-url", result.RunURL)
		require.Equal(t, "succeeded", result.Status)

		require.True(t, didOpenURL)

		require.Contains(t, s.mockStdout.String(), "Image push succeeded!")
		require.Contains(t, s.mockStdout.String(), "Pushing image from task: some-task-id\nregistry.com/repo:latest\nregistry.com/repo:17.1\n")
		require.Contains(t, s.mockStderr.String(), "Starting...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("prefers environment variable credentials over docker credentials", func(t *testing.T) {
		s := setupTest(t)

		t.Setenv("RWX_PUSH_USERNAME", "env-username")
		t.Setenv("RWX_PUSH_PASSWORD", "env-password")

		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}
		s.mockAPI.MockTaskIDStatus = func(cfg api.TaskIDStatusConfig) (api.TaskStatusResult, error) {
			return api.TaskStatusResult{Status: &api.TaskStatus{Result: "succeeded"}, Polling: api.PollingResult{Completed: true}}, nil
		}
		s.mockAPI.MockStartImagePush = func(cfg api.StartImagePushConfig) (api.StartImagePushResult, error) {
			require.Equal(t, "some-task-id", cfg.TaskID)
			require.Equal(t, "registry.com", cfg.Image.Registry)
			require.Equal(t, "repo", cfg.Image.Repository)
			require.ElementsMatch(t, []string{"latest", "17.1"}, cfg.Image.Tags)
			require.Equal(t, "env-username", cfg.Credentials.Username)
			require.Equal(t, "env-password", cfg.Credentials.Password)
			require.Equal(t, "zstd", cfg.Compression)

			return api.StartImagePushResult{PushID: "some-push-id", RunURL: "some-run-url"}, nil
		}
		s.mockAPI.MockImagePushStatus = func(pushID string) (api.ImagePushStatusResult, error) {
			require.Equal(t, "some-push-id", pushID)

			return api.ImagePushStatusResult{Status: "succeeded"}, nil
		}

		didOpenURL := false
		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				didOpenURL = true
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "some-push-id", result.PushID)
		require.Equal(t, "some-run-url", result.RunURL)
		require.Equal(t, "succeeded", result.Status)

		require.True(t, didOpenURL)

		require.Contains(t, s.mockStdout.String(), "Image push succeeded!")
		require.Contains(t, s.mockStdout.String(), "Pushing image from task: some-task-id\nregistry.com/repo:latest\nregistry.com/repo:17.1\n")
		require.Contains(t, s.mockStderr.String(), "Starting...")
		require.Contains(t, s.mockStderr.String(), "Waiting for image push to finish...")
	})

	t.Run("errors if only RWX_PUSH_USERNAME is set", func(t *testing.T) {
		s := setupTest(t)

		t.Setenv("RWX_PUSH_USERNAME", "env-username")

		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}
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

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.ErrorContains(t, err, "RWX_PUSH_PASSWORD must be set if RWX_PUSH_USERNAME is set")
	})

	t.Run("errors if only RWX_PUSH_PASSWORD is set", func(t *testing.T) {
		s := setupTest(t)

		t.Setenv("RWX_PUSH_PASSWORD", "env-password")

		s.mockDocker.GetAuthConfigFunc = func(host string) (types.AuthConfig, error) {
			return types.AuthConfig{Username: "my-username", Password: "my-password"}, nil
		}
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

		cfg := cli.ImagePushConfig{
			TaskID: "some-task-id",
			References: []reference.Named{
				ReferenceMustParse(t, "registry.com/repo:latest"),
				ReferenceMustParse(t, "registry.com/repo:17.1"),
			},
			Compression: "zstd",
			JSON:        false,
			Wait:        true,
			OpenURL: func(url string) error {
				require.Equal(t, "some-run-url", url)
				return nil
			},
		}

		result, err := s.service.ImagePush(cfg)

		require.Nil(t, result)
		require.ErrorContains(t, err, "RWX_PUSH_USERNAME must be set if RWX_PUSH_PASSWORD is set")
	})
}
