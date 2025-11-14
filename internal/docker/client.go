package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/cli/cli/command"
	cliTypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
)

type Client interface {
	GetAuthConfig(string) (cliTypes.AuthConfig, error)
	Pull(context.Context, string, cliTypes.AuthConfig) error
	Tag(context.Context, string, string) error
}

type realDockerCLI struct {
	*command.DockerCli
}

func (r realDockerCLI) GetAuthConfig(registry string) (cliTypes.AuthConfig, error) {
	return r.ConfigFile().GetAuthConfig(registry)
}

func (r realDockerCLI) Pull(ctx context.Context, imageRef string, authConfig cliTypes.AuthConfig) error {
	encodedAuth, err := encodeAuthConfig(authConfig)
	if err != nil {
		return fmt.Errorf("unable to encode auth config: %w", err)
	}

	responseBody, err := r.Client().ImagePull(ctx, imageRef, image.PullOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return fmt.Errorf("unable to pull image: %w", err)
	}
	defer responseBody.Close()

	_, err = io.Copy(io.Discard, responseBody)
	if err != nil {
		return fmt.Errorf("error reading pull response: %w", err)
	}

	return nil
}

func (r realDockerCLI) Tag(ctx context.Context, source, target string) error {
	err := r.Client().ImageTag(ctx, source, target)
	if err != nil {
		return fmt.Errorf("unable to tag image: %w", err)
	}
	return nil
}

func New() (Client, error) {
	cli, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("unable to make a new docker CLI: %w", err)
	}

	if err := cli.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, fmt.Errorf("unable to initialize the docker CLI: %w", err)
	}

	return realDockerCLI{cli}, nil
}

func encodeAuthConfig(authConfig cliTypes.AuthConfig) (string, error) {
	encodedJSON, err := json.Marshal(registry.AuthConfig{
		Username:      authConfig.Username,
		Password:      authConfig.Password,
		ServerAddress: authConfig.ServerAddress,
	})
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}
