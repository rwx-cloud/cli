package mocks

import (
	"context"

	cliTypes "github.com/docker/cli/cli/config/types"
)

type DockerClient struct {
	GetAuthConfigFunc func(string) (cliTypes.AuthConfig, error)
	PullFunc          func(context.Context, string, cliTypes.AuthConfig, bool) error
	TagFunc           func(context.Context, string, string) error
	RegistryValue     string
	PasswordValue     string
}

func (m *DockerClient) GetAuthConfig(registry string) (cliTypes.AuthConfig, error) {
	if m.GetAuthConfigFunc != nil {
		return m.GetAuthConfigFunc(registry)
	}
	return cliTypes.AuthConfig{}, nil
}

func (m *DockerClient) Pull(ctx context.Context, imageRef string, authConfig cliTypes.AuthConfig, quiet bool) error {
	if m.PullFunc != nil {
		return m.PullFunc(ctx, imageRef, authConfig, quiet)
	}
	return nil
}

func (m *DockerClient) Tag(ctx context.Context, source, target string) error {
	if m.TagFunc != nil {
		return m.TagFunc(ctx, source, target)
	}
	return nil
}

func (m *DockerClient) Registry() string {
	return m.RegistryValue
}

func (m *DockerClient) Password() string {
	return m.PasswordValue
}
