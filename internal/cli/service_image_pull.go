package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	cliTypes "github.com/docker/cli/cli/config/types"
)

type ImagePullResult struct {
	ImageRef string   `json:",omitempty"`
	Tags     []string `json:",omitempty"`
}

func (s Service) ImagePull(config ImagePullConfig) (*ImagePullResult, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	whoamiResult, err := s.APIClient.Whoami()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization info: %w\nTry running `rwx login` again", err)
	}

	registry := s.DockerCLI.Registry()
	imageRef := fmt.Sprintf("%s/%s:%s", registry, whoamiResult.OrganizationSlug, config.TaskID)

	if !config.OutputJSON {
		fmt.Fprintf(s.Stdout, "Pulling image: %s\n", imageRef)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	authConfig := cliTypes.AuthConfig{
		Username:      whoamiResult.OrganizationSlug,
		Password:      s.DockerCLI.Password(),
		ServerAddress: registry,
	}

	if err := s.DockerCLI.Pull(ctx, imageRef, authConfig, config.OutputJSON); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("timeout while pulling image after %s\n\nThe image may still be available at: %s", config.Timeout, imageRef)
		}
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	if !config.OutputJSON {
		fmt.Fprintf(s.Stdout, "\nImage pulled successfully!\n")
	}

	for _, tag := range config.Tags {
		if !config.OutputJSON {
			fmt.Fprintf(s.Stdout, "Tagging image as: %s\n", tag)
		}

		if err := s.DockerCLI.Tag(ctx, imageRef, tag); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("timeout while tagging image after %s", config.Timeout)
			}
			return nil, fmt.Errorf("failed to tag image as %s: %w", tag, err)
		}
	}

	result := &ImagePullResult{
		ImageRef: imageRef,
		Tags:     config.Tags,
	}

	if config.OutputJSON {
		if err := json.NewEncoder(s.Stdout).Encode(result); err != nil {
			return nil, fmt.Errorf("unable to encode output: %w", err)
		}
	}

	return result, nil
}
