package cli

import (
	"time"

	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
)

func (s Service) GetRunStatus(cfg GetRunStatusConfig) (*GetRunStatusResult, error) {
	defer s.outputLatestVersionMessage()

	var stopSpinner func()
	if cfg.Wait && !cfg.Json {
		stopSpinner = Spin("Waiting for run to complete...", s.StdoutIsTTY, s.Stdout)
	}

	for {
		statusResult, err := s.APIClient.RunStatus(api.RunStatusConfig{RunID: cfg.RunID})
		if err != nil {
			if stopSpinner != nil {
				stopSpinner()
			}
			return nil, errors.Wrap(err, "unable to get run status")
		}

		status := ""
		if statusResult.Status != nil {
			status = statusResult.Status.Result
		}

		if !cfg.Wait || statusResult.Polling.Completed {
			if stopSpinner != nil {
				stopSpinner()
			}
			return &GetRunStatusResult{
				RunID:        cfg.RunID,
				ResultStatus: status,
				Completed:    statusResult.Polling.Completed,
			}, nil
		}

		if statusResult.Polling.BackoffMs == nil {
			if stopSpinner != nil {
				stopSpinner()
			}
			return nil, errors.New("unable to wait for run")
		}
		time.Sleep(time.Duration(*statusResult.Polling.BackoffMs) * time.Millisecond)
	}
}
