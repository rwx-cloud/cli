package main

import (
	_ "embed"
	"encoding/json"
	"regexp"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/rwx-cloud/cli/internal/captain/backend"
	"github.com/rwx-cloud/cli/internal/captain/backend/remote"
	captaincli "github.com/rwx-cloud/cli/internal/captain/cli"
	captainerrors "github.com/rwx-cloud/cli/internal/captain/errors"
	"github.com/rwx-cloud/cli/internal/captain/exec"
	captainfs "github.com/rwx-cloud/cli/internal/captain/fs"
	"github.com/rwx-cloud/cli/internal/captain/logging"
	"github.com/rwx-cloud/cli/internal/captain/parsing"
	"github.com/rwx-cloud/cli/internal/captain/providers"
	v1 "github.com/rwx-cloud/cli/internal/captain/testingschema/v1"
	"github.com/rwx-cloud/cli/internal/git"
)

var testMutuallyExclusiveParsers []parsing.Parser = []parsing.Parser{
	parsing.DotNetxUnitParser{},
	parsing.GoGinkgoParser{},
	parsing.GoTestParser{},
	parsing.JavaScriptCypressParser{},
	parsing.JavaScriptJestParser{},
	parsing.JavaScriptVitestParser{}, // Vitest MUST be after Jest as Jest _looks like_ a superset of Vitest
	parsing.JavaScriptKarmaParser{},
	parsing.JavaScriptMochaParser{},
	parsing.JavaScriptPlaywrightParser{},
	parsing.PythonPytestParser{},
	parsing.RubyRSpecParser{},
}

var testFrameworkParsers map[v1.Framework][]parsing.Parser = map[v1.Framework][]parsing.Parser{
	v1.DotNetxUnitFramework:          {parsing.DotNetxUnitParser{}},
	v1.ElixirExUnitFramework:         {parsing.ElixirExUnitParser{}},
	v1.GoGinkgoFramework:             {parsing.GoGinkgoParser{}},
	v1.GoTestFramework:               {parsing.GoTestParser{}},
	v1.JavaScriptCucumberFramework:   {parsing.JavaScriptCucumberJSONParser{}},
	v1.JavaScriptCypressFramework:    {parsing.JavaScriptCypressParser{}},
	v1.JavaScriptJestFramework:       {parsing.JavaScriptJestParser{}},
	v1.JavaScriptKarmaFramework:      {parsing.JavaScriptKarmaParser{}},
	v1.JavaScriptMochaFramework:      {parsing.JavaScriptMochaParser{}},
	v1.JavaScriptPlaywrightFramework: {parsing.JavaScriptPlaywrightParser{}},
	v1.JavaScriptVitestFramework:     {parsing.JavaScriptVitestParser{}},
	v1.JavaScriptBunFramework:        {parsing.JUnitTestsuitesParser{}, parsing.JUnitTestsuiteParser{}},
	v1.PHPUnitFramework:              {parsing.PHPUnitParser{}},
	v1.PythonPytestFramework:         {parsing.PythonPytestParser{}},
	v1.PythonUnitTestFramework:       {parsing.PythonUnitTestParser{}},
	v1.RubyCucumberFramework:         {parsing.RubyCucumberParser{}},
	v1.RubyMinitestFramework:         {parsing.RubyMinitestParser{}},
	v1.RubyRSpecFramework:            {parsing.RubyRSpecParser{}},
}

var testGenericParsers []parsing.Parser = []parsing.Parser{
	parsing.RWXParser{},
	parsing.JUnitTestsuitesParser{},
	parsing.JUnitTestsuiteParser{},
}

var invalidSuiteIDRegexp = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

type identityRecipe struct {
	Language string
	Kind     string
	Recipe   struct {
		Components []string
		Strict     bool
	}
}

//go:embed identity_recipes.json
var recipeJSON []byte

func getTestRecipes() (map[string]v1.TestIdentityRecipe, error) {
	var recipeList []identityRecipe
	recipes := make(map[string]v1.TestIdentityRecipe)

	if err := json.Unmarshal(recipeJSON, &recipeList); err != nil {
		return recipes, captainerrors.NewInternalError("unable to parse identity recipes: %s", err.Error())
	}

	for _, ir := range recipeList {
		recipes[v1.CoerceFramework(ir.Language, ir.Kind).String()] = v1.TestIdentityRecipe{
			Components: ir.Recipe.Components,
			Strict:     ir.Recipe.Strict,
		}
	}

	return recipes, nil
}

func initTestService(
	cliArgs *testCliArgs,
	providerValidator func(providers.Provider) error,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := extractTestSuiteID(&cliArgs.rootCliArgs, args); err != nil {
			return err
		}

		cfg, err := initTestConfig(cmd, *cliArgs)
		if err != nil {
			return captainerrors.WithDecoration(err)
		}

		return initTestServiceWithConfig(cmd, cfg, cliArgs.rootCliArgs.suiteID, providerValidator)
	}
}

func initTestServiceWithConfig(
	cmd *cobra.Command, cfg testConfig, suiteID string, providerValidator func(providers.Provider) error,
) error {
	err := func() error {
		if suiteID == "" {
			return captainerrors.NewConfigurationError("Invalid suite-id", "The suite ID is empty.", "")
		}

		if invalidSuiteIDRegexp.Match([]byte(suiteID)) {
			return captainerrors.NewConfigurationError(
				"Invalid suite-id",
				"A suite ID can only contain alphanumeric characters, `_` and `-`.",
				"Please make sure that the ID doesn't contain any special characters.",
			)
		}

		logger := logging.NewProductionLogger()
		if cfg.Output.Debug {
			logger = logging.NewDebugLogger()
		}

		apiClient, err := makeTestAPIClient(cfg, providerValidator, logger, suiteID)
		if err != nil {
			return captainerrors.Wrap(err, "unable to create API client")
		}

		recipes, err := getTestRecipes()
		if err != nil {
			return captainerrors.Wrap(err, "unable to retrieve test identity recipes")
		}

		var parseConfig parsing.Config
		if suiteConfig, ok := cfg.TestSuites[suiteID]; ok {
			parseConfig = parsing.Config{
				ProvidedFrameworkKind:     suiteConfig.Results.Framework,
				ProvidedFrameworkLanguage: suiteConfig.Results.Language,
				FailOnDuplicateTestID:     suiteConfig.FailOnDuplicateTestID,
				MutuallyExclusiveParsers:  testMutuallyExclusiveParsers,
				FrameworkParsers:          testFrameworkParsers,
				GenericParsers:            testGenericParsers,
				Logger:                    logger,
				IdentityRecipes:           recipes,
			}
		}

		if err := parseConfig.Validate(); err != nil {
			return captainerrors.Wrap(err, "invalid parser config")
		}

		captain := captaincli.Service{
			API:         apiClient,
			Log:         logger,
			FileSystem:  captainfs.Local{},
			TaskRunner:  exec.Local{},
			ParseConfig: parseConfig,
		}

		if err := captaincli.SetService(cmd, captain); err != nil {
			return captainerrors.WithStack(err)
		}

		return nil
	}()
	if err != nil {
		return captainerrors.WithDecoration(err)
	}
	return nil
}

func unsafeInitTestParsingOnly(cliArgs *testCliArgs) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := func() error {
			cliArgs.rootCliArgs.positionalArgs = args
			if cliArgs.rootCliArgs.suiteID == "" {
				cliArgs.rootCliArgs.suiteID = "placeholder"
			}

			cfg, err := initTestConfig(cmd, *cliArgs)
			if err != nil {
				return captainerrors.WithStack(err)
			}

			logger := logging.NewProductionLogger()
			if cfg.Output.Debug {
				logger = logging.NewDebugLogger()
			}

			recipes, err := getTestRecipes()
			if err != nil {
				return captainerrors.Wrap(err, "unable to retrieve test identity recipes")
			}

			var parseConfig parsing.Config
			if suiteConfig, ok := cfg.TestSuites[cliArgs.rootCliArgs.suiteID]; ok {
				parseConfig = parsing.Config{
					ProvidedFrameworkKind:     suiteConfig.Results.Framework,
					ProvidedFrameworkLanguage: suiteConfig.Results.Language,
					FailOnDuplicateTestID:     suiteConfig.FailOnDuplicateTestID,
					MutuallyExclusiveParsers:  testMutuallyExclusiveParsers,
					FrameworkParsers:          testFrameworkParsers,
					GenericParsers:            testGenericParsers,
					Logger:                    logger,
					IdentityRecipes:           recipes,
				}
			}

			if err := parseConfig.Validate(); err != nil {
				return captainerrors.Wrap(err, "invalid parser config")
			}

			captain := captaincli.Service{
				Log:         logger,
				FileSystem:  captainfs.Local{},
				ParseConfig: parseConfig,
			}
			if err := captaincli.SetService(cmd, captain); err != nil {
				return captainerrors.WithStack(err)
			}

			return nil
		}()
		if err != nil {
			return captainerrors.WithDecoration(err)
		}
		return nil
	}
}

func makeTestAPIClient(
	cfg testConfig, providerValidator func(providers.Provider) error, logger *zap.SugaredLogger, suiteID string,
) (backend.Client, error) {
	gitClient := &git.Client{Binary: "git", Dir: "."}
	cfg.ProvidersEnv.Generic.PopulateFromGit(gitClient)

	if cfg.Secrets.APIToken == "" {
		return nil, captainerrors.NewConfigurationError(
			"Missing access token",
			"rwx test requires an RWX access token to communicate with RWX Cloud.",
			"Set the RWX_ACCESS_TOKEN environment variable or run 'rwx login' to authenticate.",
		)
	}

	provider, err := cfg.ProvidersEnv.MakeProvider()
	if err != nil {
		return nil, captainerrors.Wrap(err, "failed to construct provider")
	}
	if err = providerValidator(provider); err != nil {
		return nil, err
	}

	client, err := remote.NewClient(remote.ClientConfig{
		Debug:    cfg.Output.Debug,
		Host:     cfg.Cloud.APIHost,
		Insecure: cfg.Cloud.Insecure,
		Log:      logger,
		Token:    cfg.Secrets.APIToken,
		Provider: provider,
	})
	return client, captainerrors.WithStack(err)
}
