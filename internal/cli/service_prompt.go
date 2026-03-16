package cli

import "github.com/rwx-cloud/rwx/internal/api"

type GetRunPromptConfig struct {
	RunID string
	All   bool
	Json  bool
	Page  int
}

type GetRunPromptResult struct {
	Prompt        string
	Tasks         []api.RunPromptTask
	Page          int
	HasMore       bool
	RunInProgress bool
}

func (s Service) GetRunPrompt(cfg GetRunPromptConfig) (*GetRunPromptResult, error) {
	result, err := s.APIClient.GetRunPrompt(api.GetRunPromptConfig{
		RunID: cfg.RunID,
		All:   cfg.All,
		Json:  cfg.Json,
		Page:  cfg.Page,
	})
	if err != nil {
		return nil, err
	}
	return &GetRunPromptResult{Prompt: result.Prompt, Tasks: result.Tasks, Page: result.Page, HasMore: result.HasMore, RunInProgress: result.RunInProgress}, nil
}
