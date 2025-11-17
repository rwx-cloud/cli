package cli

import (
	"context"
	"errors"
	"fmt"

	cliTypes "github.com/docker/cli/cli/config/types"
)

func (s Service) PullImage(config PullImageConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}

	whoamiResult, err := s.APIClient.Whoami()
	if err != nil {
		return fmt.Errorf("failed to get organization info: %w\nTry running `rwx login` again", err)
	}

	registry := s.DockerCLI.Registry()
	imageRef := fmt.Sprintf("%s/%s:%s", registry, whoamiResult.OrganizationSlug, config.TaskID)

	fmt.Fprintf(s.Stdout, "Pulling image: %s\n", imageRef)

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	authConfig := cliTypes.AuthConfig{
		Username:      whoamiResult.OrganizationSlug,
		Password:      s.DockerCLI.Password(),
		ServerAddress: registry,
	}

	if err := s.DockerCLI.Pull(ctx, imageRef, authConfig); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("timeout while pulling image after %s\n\nThe image may still be available at: %s", config.Timeout, imageRef)
		}
		return fmt.Errorf("failed to pull image: %w", err)
	}

	fmt.Fprintf(s.Stdout, "\nImage pulled successfully!\n")

	for _, tag := range config.Tags {
		fmt.Fprintf(s.Stdout, "Tagging image as: %s\n", tag)

		if err := s.DockerCLI.Tag(ctx, imageRef, tag); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("timeout while tagging image after %s", config.Timeout)
			}
			return fmt.Errorf("failed to tag image as %s: %w", tag, err)
		}
	}

	return nil
}
