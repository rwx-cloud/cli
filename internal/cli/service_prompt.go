package cli

type GetRunPromptResult struct {
	Prompt string
}

func (s Service) GetRunPrompt(runID string) (*GetRunPromptResult, error) {
	prompt, err := s.APIClient.GetRunPrompt(runID)
	if err != nil {
		return nil, err
	}
	return &GetRunPromptResult{Prompt: prompt}, nil
}
