package cli

import "github.com/rwx-cloud/rwx/internal/api"

type GetRunPromptConfig struct {
	RunID string
	All   bool
	Json  bool
}

type GetRunPromptResult struct {
	Prompt string
	Tasks  []api.RunPromptTask
}

func (s Service) GetRunPrompt(cfg GetRunPromptConfig) (*GetRunPromptResult, error) {
	result, err := s.APIClient.GetRunPrompt(api.GetRunPromptConfig{
		RunID: cfg.RunID,
		All:   cfg.All,
		Json:  cfg.Json,
	})
	if err != nil {
		return nil, err
	}
	return &GetRunPromptResult{Prompt: result.Prompt, Tasks: result.Tasks}, nil
}
