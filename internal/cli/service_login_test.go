package cli_test

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/rwx-research/mint-cli/internal/accesstoken"
	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestService_LoggingIn(t *testing.T) {
	var tokenBackend accesstoken.Backend

	beforeEachSetup := func(t *testing.T) {
		var err error
		tokenBackend, err = accesstoken.NewMemoryBackend()
		require.NoError(t, err)
	}

	t.Run("when unable to obtain an auth code", func(t *testing.T) {
		// Setup
		s := setupTest(t)
		beforeEachSetup(t)

		s.mockAPI.MockObtainAuthCode = func(oacc api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
			require.Equal(t, "some-device", oacc.Code.DeviceName)
			return nil, errors.New("error in obtain auth code")
		}

		// returns an error
		err := s.service.Login(cli.LoginConfig{
			DeviceName:         "some-device",
			AccessTokenBackend: tokenBackend,
			OpenUrl: func(url string) error {
				require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
				return nil
			},
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "error in obtain auth code")
	})

	t.Run("with an auth code created", func(t *testing.T) {
		t.Run("when polling results in authorized", func(t *testing.T) {
			// Setup
			s := setupTest(t)
			beforeEachSetup(t)

			s.mockAPI.MockObtainAuthCode = func(oacc api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
				require.Equal(t, "some-device", oacc.Code.DeviceName)
				return &api.ObtainAuthCodeResult{
					AuthorizationUrl: "https://cloud.local/_/auth/code?code=your-code",
					TokenUrl:         "https://cloud.local/api/auth/codes/code-uuid/token",
				}, nil
			}

			pollCounter := 0
			s.mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
				require.Equal(t, "https://cloud.local/api/auth/codes/code-uuid/token", tokenUrl)

				if pollCounter > 1 {
					pollCounter++
					return &api.AcquireTokenResult{State: "authorized", Token: "your-token"}, nil
				} else {
					pollCounter++
					return &api.AcquireTokenResult{State: "pending"}, nil
				}
			}

			t.Run("does not error", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})

				require.NoError(t, err)
			})

			t.Run("stores the token", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.NoError(t, err)

				token, err := tokenBackend.Get()
				require.NoError(t, err)
				require.Equal(t, "your-token", token)
			})

			t.Run("indicates success and help in case the browser does not open", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.NoError(t, err)

				require.Contains(t, s.mockStdout.String(), "https://cloud.local/_/auth/code?code=your-code")
				require.Contains(t, s.mockStdout.String(), "Authorized!")
			})

			t.Run("attempts to open the authorization URL, but doesn't care if it fails", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return errors.New("couldn't open it")
					},
				})
				require.NoError(t, err)

				require.Contains(t, s.mockStdout.String(), "https://cloud.local/_/auth/code?code=your-code")
				require.Contains(t, s.mockStdout.String(), "Authorized!")
			})
		})

		t.Run("when polling results in consumed", func(t *testing.T) {
			// Setup
			s := setupTest(t)
			beforeEachSetup(t)

			s.mockAPI.MockObtainAuthCode = func(oacc api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
				require.Equal(t, "some-device", oacc.Code.DeviceName)
				return &api.ObtainAuthCodeResult{
					AuthorizationUrl: "https://cloud.local/_/auth/code?code=your-code",
					TokenUrl:         "https://cloud.local/api/auth/codes/code-uuid/token",
				}, nil
			}

			pollCounter := 0
			s.mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
				require.Equal(t, "https://cloud.local/api/auth/codes/code-uuid/token", tokenUrl)

				if pollCounter > 1 {
					pollCounter++
					return &api.AcquireTokenResult{State: "consumed"}, nil
				} else {
					pollCounter++
					return &api.AcquireTokenResult{State: "pending"}, nil
				}
			}

			t.Run("errors", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})

				require.Error(t, err)
				require.Contains(t, err.Error(), "already been used")
			})

			t.Run("does not store the token", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.Error(t, err)

				token, err := tokenBackend.Get()
				require.NoError(t, err)
				require.Equal(t, "", token)
			})

			t.Run("does not indicate success, but still helps in case the browser does not open", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.Error(t, err)

				require.Contains(t, s.mockStdout.String(), "https://cloud.local/_/auth/code?code=your-code")
				require.NotContains(t, s.mockStdout.String(), "Authorized!")
			})
		})

		t.Run("when polling results in expired", func(t *testing.T) {
			// Setup
			s := setupTest(t)
			beforeEachSetup(t)

			s.mockAPI.MockObtainAuthCode = func(oacc api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
				require.Equal(t, "some-device", oacc.Code.DeviceName)
				return &api.ObtainAuthCodeResult{
					AuthorizationUrl: "https://cloud.local/_/auth/code?code=your-code",
					TokenUrl:         "https://cloud.local/api/auth/codes/code-uuid/token",
				}, nil
			}

			pollCounter := 0
			s.mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
				require.Equal(t, "https://cloud.local/api/auth/codes/code-uuid/token", tokenUrl)

				if pollCounter > 1 {
					pollCounter++
					return &api.AcquireTokenResult{State: "expired"}, nil
				} else {
					pollCounter++
					return &api.AcquireTokenResult{State: "pending"}, nil
				}
			}

			t.Run("errors", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})

				require.Error(t, err)
				require.Contains(t, err.Error(), "has expired")
			})

			t.Run("does not store the token", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.Error(t, err)

				token, err := tokenBackend.Get()
				require.NoError(t, err)
				require.Equal(t, "", token)
			})

			t.Run("does not indicate success, but still helps in case the browser does not open", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.Error(t, err)

				require.Contains(t, s.mockStdout.String(), "https://cloud.local/_/auth/code?code=your-code")
				require.NotContains(t, s.mockStdout.String(), "Authorized!")
			})
		})

		t.Run("when polling results in something else", func(t *testing.T) {
			// Setup
			s := setupTest(t)
			beforeEachSetup(t)

			s.mockAPI.MockObtainAuthCode = func(oacc api.ObtainAuthCodeConfig) (*api.ObtainAuthCodeResult, error) {
				require.Equal(t, "some-device", oacc.Code.DeviceName)
				return &api.ObtainAuthCodeResult{
					AuthorizationUrl: "https://cloud.local/_/auth/code?code=your-code",
					TokenUrl:         "https://cloud.local/api/auth/codes/code-uuid/token",
				}, nil
			}

			pollCounter := 0
			s.mockAPI.MockAcquireToken = func(tokenUrl string) (*api.AcquireTokenResult, error) {
				require.Equal(t, "https://cloud.local/api/auth/codes/code-uuid/token", tokenUrl)

				if pollCounter > 1 {
					pollCounter++
					return &api.AcquireTokenResult{State: "unexpected-state-here-uh-oh"}, nil
				} else {
					pollCounter++
					return &api.AcquireTokenResult{State: "pending"}, nil
				}
			}

			t.Run("errors", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})

				require.Error(t, err)
				require.Contains(t, err.Error(), "is in an unexpected state")
			})

			t.Run("does not store the token", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.Error(t, err)

				token, err := tokenBackend.Get()
				require.NoError(t, err)
				require.Equal(t, "", token)
			})

			t.Run("does not indicate success, but still helps in case the browser does not open", func(t *testing.T) {
				err := s.service.Login(cli.LoginConfig{
					DeviceName:         "some-device",
					AccessTokenBackend: tokenBackend,
					OpenUrl: func(url string) error {
						require.Equal(t, "https://cloud.local/_/auth/code?code=your-code", url)
						return nil
					},
				})
				require.Error(t, err)

				require.Contains(t, s.mockStdout.String(), "https://cloud.local/_/auth/code?code=your-code")
				require.NotContains(t, s.mockStdout.String(), "Authorized!")
			})
		})
	})
}
