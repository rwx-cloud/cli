package cli

import (
	"github.com/rwx-cloud/cli/internal/api"
	"github.com/rwx-cloud/cli/internal/errors"
)

func (s Service) InitiateDispatch(cfg InitiateDispatchConfig) (*api.InitiateDispatchResult, error) {
	defer s.outputLatestVersionMessage()
	err := cfg.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	dispatchResult, err := s.APIClient.InitiateDispatch(api.InitiateDispatchConfig{
		DispatchKey: cfg.DispatchKey,
		Params:      cfg.Params,
		Ref:         cfg.Ref,
		Title:       cfg.Title,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initiate dispatch")
	}

	return dispatchResult, nil
}

func (s Service) GetDispatch(cfg GetDispatchConfig) ([]GetDispatchRun, error) {
	defer s.outputLatestVersionMessage()
	dispatchResult, err := s.APIClient.GetDispatch(api.GetDispatchConfig{
		DispatchId: cfg.DispatchId,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get dispatch")
	}

	if dispatchResult.Status == "not_ready" {
		return nil, errors.ErrRetry
	}

	if dispatchResult.Status == "error" {
		if dispatchResult.Error == "" {
			return nil, errors.New("Failed to get dispatch")
		}
		return nil, errors.New(dispatchResult.Error)
	}

	if len(dispatchResult.Runs) == 0 {
		return nil, errors.New("No runs were created as a result of this dispatch")
	}

	runs := make([]GetDispatchRun, len(dispatchResult.Runs))
	for i, run := range dispatchResult.Runs {
		runs[i] = GetDispatchRun{RunId: run.RunId, RunUrl: run.RunUrl}
	}

	return runs, nil
}
