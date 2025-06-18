package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rwx-research/mint-cli/internal/api"
	"github.com/rwx-research/mint-cli/internal/cli"
	"github.com/rwx-research/mint-cli/internal/messages"
	"github.com/stretchr/testify/require"
)

func TestService_Linting(t *testing.T) {
	setupLintTest := func(t *testing.T) (*testSetup, cli.LintConfig) {
		s := setupTest(t)

		err := os.MkdirAll(filepath.Join(s.tmp, "some/path/to/.mint"), 0o755)
		require.NoError(t, err)
		err = os.Chdir(filepath.Join(s.tmp, "some/path/to"))
		require.NoError(t, err)

		lintConfig := cli.LintConfig{OutputFormat: cli.LintOutputNone}

		return s, lintConfig
	}

	t.Run("with multiple errors", func(t *testing.T) {
		s, lintConfig := setupLintTest(t)

		err := os.WriteFile(".mint/base.yml", []byte(".mint/base.yml contents"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(".mint/base.json", []byte(".mint/base.json contents"), 0o644)
		require.NoError(t, err)

		s.mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
			require.Len(t, cfg.TaskDefinitions, 1)
			return &api.LintResult{
				Problems: []api.LintProblem{
					{Severity: "error", Message: "message 1\nmessage 1a", FileName: ".mint/base.yml", Line: api.NewNullInt(11), Column: api.NewNullInt(22), Advice: "advice 1\nadvice 1a"},
					{Severity: "error", Message: "message 2\nmessage 2a", FileName: ".mint/base.yml", Line: api.NewNullInt(15), Column: api.NewNullInt(4)},
					{Severity: "warning", Message: "message 3", FileName: ".mint/base.yml", Line: api.NewNullInt(2), Column: api.NewNullInt(6), Advice: "advice 3\nadvice 3a"},
					{Severity: "warning", Message: "message 4", FileName: ".mint/base.yml", Line: api.NullInt{IsNull: true}, Column: api.NullInt{IsNull: true}},
				},
			}, nil
		}

		t.Run("using oneline output", func(t *testing.T) {
			lintConfig.OutputFormat = cli.LintOutputOneLine

			// lists only files
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, `error   .mint/base.yml:11:22 - message 1 message 1a
error   .mint/base.yml:15:4 - message 2 message 2a
warning .mint/base.yml:2:6 - message 3
warning .mint/base.yml - message 4
`, s.mockStdout.String())
		})

		t.Run("using multiline output", func(t *testing.T) {
			s.mockStdout.Reset()
			lintConfig.OutputFormat = cli.LintOutputMultiLine

			// lists all the data from the problem
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, `
.mint/base.yml:11:22  [error]
message 1
message 1a
advice 1
advice 1a

.mint/base.yml:15:4  [error]
message 2
message 2a

.mint/base.yml:2:6  [warning]
message 3
advice 3
advice 3a

.mint/base.yml  [warning]
message 4

Checked 1 file and found 4 problems.
`, s.mockStdout.String())
		})

		t.Run("using none output", func(t *testing.T) {
			s.mockStdout.Reset()
			lintConfig.OutputFormat = cli.LintOutputNone

			// doesn't output
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, "", s.mockStdout.String())
		})
	})

	t.Run("with multiple errors including stack traces", func(t *testing.T) {
		s, lintConfig := setupLintTest(t)

		err := os.WriteFile(".mint/base.yml", []byte(".mint/base.yml contents"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(".mint/base.json", []byte(".mint/base.json contents"), 0o644)
		require.NoError(t, err)

		s.mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
			require.Len(t, cfg.TaskDefinitions, 1)
			return &api.LintResult{
				Problems: []api.LintProblem{
					{
						Severity: "error",
						Message:  "message 1\nmessage 1a",
						StackTrace: []messages.StackEntry{
							{
								FileName: ".mint/base.yml",
								Line:     11,
								Column:   22,
							},
						},
						Frame:  "  4 |     run: echo hi\n> 5 |     bad: true\n    |     ^\n  6 |     env:\n  7 |       A:",
						Advice: "advice 1\nadvice 1a",
					},
					{
						Severity: "error",
						Message:  "message 2\nmessage 2a",
						StackTrace: []messages.StackEntry{
							{
								FileName: ".mint/base.yml",
								Line:     22,
								Column:   11,
								Name:     "*alias",
							},
							{
								FileName: ".mint/base.yml",
								Line:     5,
								Column:   22,
							},
						},
					},
					{
						Severity: "warning",
						Message:  "message 3",
						StackTrace: []messages.StackEntry{
							{
								FileName: ".mint/base.yml",
								Line:     2,
								Column:   6,
							},
						},
						Advice: "advice 3\nadvice 3a",
					},
					{
						Severity: "warning",
						Message:  "message 4",
						StackTrace: []messages.StackEntry{
							{
								FileName: ".mint/base.yml",
								Line:     7,
								Column:   9,
							},
						},
					},
				},
			}, nil
		}

		t.Run("using oneline output", func(t *testing.T) {
			lintConfig.OutputFormat = cli.LintOutputOneLine

			// lists only files
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, `error   .mint/base.yml:11:22 - message 1 message 1a
error   .mint/base.yml:5:22 - message 2 message 2a
warning .mint/base.yml:2:6 - message 3
warning .mint/base.yml:7:9 - message 4
`, s.mockStdout.String())
		})

		t.Run("using multiline output", func(t *testing.T) {
			s.mockStdout.Reset()
			lintConfig.OutputFormat = cli.LintOutputMultiLine

			// lists all the data from the problem
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, `
[error] message 1
message 1a
  4 |     run: echo hi
> 5 |     bad: true
    |     ^
  6 |     env:
  7 |       A:
  at .mint/base.yml:11:22
advice 1
advice 1a

[error] message 2
message 2a
  at .mint/base.yml:5:22
  at *alias (.mint/base.yml:22:11)

[warning] message 3
  at .mint/base.yml:2:6
advice 3
advice 3a

[warning] message 4
  at .mint/base.yml:7:9

Checked 1 file and found 4 problems.
`, s.mockStdout.String())
		})

		t.Run("using none output", func(t *testing.T) {
			s.mockStdout.Reset()
			lintConfig.OutputFormat = cli.LintOutputNone

			// doesn't output
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, "", s.mockStdout.String())
		})
	})

	t.Run("with no errors", func(t *testing.T) {
		s, lintConfig := setupLintTest(t)

		err := os.WriteFile(".mint/base.yml", []byte(".mint/base.yml contents"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(".mint/base.json", []byte(".mint/base.json contents"), 0o644)
		require.NoError(t, err)

		s.mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
			require.Len(t, cfg.TaskDefinitions, 1)
			return &api.LintResult{}, nil
		}

		t.Run("using oneline output", func(t *testing.T) {
			lintConfig.OutputFormat = cli.LintOutputOneLine

			// doesn't output
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, "", s.mockStdout.String())
		})

		t.Run("using multiline output", func(t *testing.T) {
			s.mockStdout.Reset()
			lintConfig.OutputFormat = cli.LintOutputMultiLine

			// outputs check counts
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, "\nChecked 1 file and found 0 problems.\n", s.mockStdout.String())
		})

		t.Run("using none output", func(t *testing.T) {
			s.mockStdout.Reset()
			lintConfig.OutputFormat = cli.LintOutputNone

			// doesn't output
			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, "", s.mockStdout.String())
		})
	})

	t.Run("with snippets", func(t *testing.T) {
		s, lintConfig := setupLintTest(t)

		err := os.WriteFile(".mint/base1.yml", []byte(".mint/base1.yml contents"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(".mint/base2.yml", []byte(".mint/base2.yml contents"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(".mint/_snippet1.yml", []byte(".mint/_snippet1.yml contents"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(".mint/_snippet2.yml", []byte(".mint/_snippet2.yml contents"), 0o644)
		require.NoError(t, err)

		lintConfig.OutputFormat = cli.LintOutputOneLine

		t.Run("without targeting", func(t *testing.T) {
			// doesn't target the snippets
			s.mockAPI.MockLint = func(cfg api.LintConfig) (*api.LintResult, error) {
				runDefinitionPaths := make([]string, len(cfg.TaskDefinitions))
				for i, runDefinitionPath := range cfg.TaskDefinitions {
					runDefinitionPaths[i] = runDefinitionPath.Path
				}
				require.ElementsMatch(t, []string{".mint/base1.yml", ".mint/base2.yml", ".mint/_snippet1.yml", ".mint/_snippet2.yml"}, runDefinitionPaths)
				require.ElementsMatch(t, []string{".mint/base1.yml", ".mint/base2.yml"}, cfg.TargetPaths)
				return &api.LintResult{}, nil
			}

			_, err := s.service.Lint(lintConfig)
			require.NoError(t, err)
			require.Equal(t, "", s.mockStdout.String())
		})
	})
}
