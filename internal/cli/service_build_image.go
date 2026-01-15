package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	cliTypes "github.com/docker/cli/cli/config/types"
	"github.com/rwx-cloud/cli/internal/api"
)

func (s Service) BuildImage(config BuildImageConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}

	runResult, err := s.InitiateRun(InitiateRunConfig{
		InitParameters: config.InitParameters,
		RwxDirectory:   config.RwxDirectory,
		MintFilePath:   config.MintFilePath,
		NoCache:        config.NoCache,
		TargetedTasks:  []string{config.TargetTaskKey},
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(s.Stdout, "Building image for %s\n", config.TargetTaskKey)
	fmt.Fprintf(s.Stdout, "Run URL: %s\n\n", runResult.RunURL)

	if err := config.OpenURL(runResult.RunURL); err != nil {
		return fmt.Errorf("failed to open URL: %w", err)
	}

	stopSpinner := Spin(
		"Polling for build completion...",
		s.StderrIsTTY,
		s.Stderr,
	)

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	var taskID string
	succeeded := false
	for !succeeded {
		select {
		case <-ctx.Done():
			stopSpinner()
			return fmt.Errorf("timeout waiting for build to complete after %s\n\nThe build may still be running. Check the status at: %s", config.Timeout, runResult.RunURL)
		default:
		}

		result, err := s.APIClient.TaskKeyStatus(api.TaskKeyStatusConfig{
			RunID:   runResult.RunId,
			TaskKey: config.TargetTaskKey,
		})
		if err != nil {
			stopSpinner()
			return fmt.Errorf("failed to get build status: %w", err)
		}

		if result.Polling.Completed {
			if result.Status != nil && result.Status.Result == api.TaskStatusSucceeded {
				taskID = result.TaskID
				stopSpinner()
				fmt.Fprintf(s.Stdout, "\nBuild succeeded!\n\n")
				succeeded = true
			} else {
				stopSpinner()
				return fmt.Errorf("build failed")
			}
		} else {
			if result.Polling.BackoffMs == nil {
				stopSpinner()
				return fmt.Errorf("build failed")
			}
			time.Sleep(time.Duration(*result.Polling.BackoffMs) * time.Millisecond)
		}
	}

	whoamiResult, err := s.APIClient.Whoami()
	if err != nil {
		return fmt.Errorf("failed to get organization info: %w\nTry running `rwx login` again", err)
	}

	registry := s.DockerCLI.Registry()
	imageRef := fmt.Sprintf("%s/%s:%s", registry, whoamiResult.OrganizationSlug, taskID)

	if config.NoPull {
		fmt.Fprintf(s.Stdout, "Image available at: %s\n", imageRef)
		return nil
	}

	fmt.Fprintf(s.Stdout, "Pulling image: %s\n", imageRef)

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

	if len(config.PushToReferences) > 0 {
		fmt.Fprintf(s.Stdout, "\n")

		pushConfig, err := NewPushImageConfig(
			taskID,
			config.PushToReferences,
			false,
			true,
			func(url string) error {
				fmt.Fprintf(s.Stdout, "Run URL: %s\n", url)
				return nil
			},
		)
		if err != nil {
			return err
		}

		if err := s.PushImage(pushConfig); err != nil {
			return err
		}
	}

	return nil
}
