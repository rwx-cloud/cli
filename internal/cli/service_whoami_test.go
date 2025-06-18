package cli_test

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_Whoami(t *testing.T) {
	t.Run("when outputting json", func(t *testing.T) {
		t.Run("when the request fails", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
				return nil, errors.New("uh oh can't figure out who you are")
			}

			err := s.service.Whoami(cli.WhoamiConfig{
				Json: true,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "unable to determine details about the access token")
			require.Contains(t, err.Error(), "can't figure out who you are")
		})

		t.Run("when there is an email", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
				email := "someone@rwx.com"
				return &api.WhoamiResult{
					TokenKind:        "personal_access_token",
					OrganizationSlug: "rwx",
					UserEmail:        &email,
				}, nil
			}

			err := s.service.Whoami(cli.WhoamiConfig{
				Json: true,
			})

			require.NoError(t, err)
			require.Contains(t, s.mockStdout.String(), `"token_kind": "personal_access_token"`)
			require.Contains(t, s.mockStdout.String(), `"organization_slug": "rwx"`)
			require.Contains(t, s.mockStdout.String(), `"user_email": "someone@rwx.com"`)
		})

		t.Run("when there is not an email", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
				return &api.WhoamiResult{
					TokenKind:        "organization_access_token",
					OrganizationSlug: "rwx",
				}, nil
			}

			err := s.service.Whoami(cli.WhoamiConfig{
				Json: true,
			})

			require.NoError(t, err)
			require.Contains(t, s.mockStdout.String(), `"token_kind": "organization_access_token"`)
			require.Contains(t, s.mockStdout.String(), `"organization_slug": "rwx"`)
			require.NotContains(t, s.mockStdout.String(), `"user_email"`)
		})
	})

	t.Run("when outputting plaintext", func(t *testing.T) {
		t.Run("when the request fails", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
				return nil, errors.New("uh oh can't figure out who you are")
			}

			err := s.service.Whoami(cli.WhoamiConfig{
				Json: false,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "unable to determine details about the access token")
			require.Contains(t, err.Error(), "can't figure out who you are")
		})

		t.Run("when there is an email", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
				email := "someone@rwx.com"
				return &api.WhoamiResult{
					TokenKind:        "personal_access_token",
					OrganizationSlug: "rwx",
					UserEmail:        &email,
				}, nil
			}

			err := s.service.Whoami(cli.WhoamiConfig{
				Json: false,
			})

			require.NoError(t, err)
			require.Contains(t, s.mockStdout.String(), "Token Kind: personal access token")
			require.Contains(t, s.mockStdout.String(), "Organization: rwx")
			require.Contains(t, s.mockStdout.String(), "User: someone@rwx.com")
		})

		t.Run("when there is not an email", func(t *testing.T) {
			s := setupTest(t)

			s.mockAPI.MockWhoami = func() (*api.WhoamiResult, error) {
				return &api.WhoamiResult{
					TokenKind:        "organization_access_token",
					OrganizationSlug: "rwx",
				}, nil
			}

			err := s.service.Whoami(cli.WhoamiConfig{
				Json: false,
			})

			require.NoError(t, err)
			require.Contains(t, s.mockStdout.String(), "Token Kind: organization access token")
			require.Contains(t, s.mockStdout.String(), "Organization: rwx")
			require.NotContains(t, s.mockStdout.String(), "User:")
		})
	})
}
