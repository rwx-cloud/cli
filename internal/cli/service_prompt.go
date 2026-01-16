package cli

func (s Service) GetRunPrompt(runID string) (string, error) {
	return s.APIClient.GetRunPrompt(runID)
}
