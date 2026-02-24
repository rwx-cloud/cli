package cli

import "github.com/rwx-cloud/cli/internal/docstoken"

func (s Service) resolveDocsToken() string {
	if s.DocsTokenBackend == nil || s.AccessTokenBackend == nil {
		return ""
	}

	currentAuthToken, err := s.AccessTokenBackend.Get()
	if err != nil || currentAuthToken == "" {
		return ""
	}

	cached, err := s.DocsTokenBackend.Get()
	if err == nil && cached.Token != "" && cached.AuthToken == currentAuthToken {
		return cached.Token
	}

	result, err := s.APIClient.CreateDocsToken()
	if err != nil {
		return ""
	}

	_ = s.DocsTokenBackend.Set(docstoken.DocsToken{
		Token:     result.Token,
		AuthToken: currentAuthToken,
	})

	return result.Token
}
