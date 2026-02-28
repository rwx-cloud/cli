package main

import (
	_ "embed"
	"encoding/json"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/rwx-cloud/cli/internal/captain/backend"
	"github.com/rwx-cloud/cli/internal/captain/backend/local"
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
	wrapError := func(a backend.Client, b error) (backend.Client, error) {
		return a, captainerrors.WithStack(b)
	}
	gitClient := &git.Client{Binary: "git", Dir: "."}
	cfg.ProvidersEnv.Generic.PopulateFromGit(gitClient)

	if cfg.Secrets.APIToken != "" && !cfg.Cloud.Disabled {
		provider, err := cfg.ProvidersEnv.MakeProvider()
		if err != nil {
			return nil, captainerrors.Wrap(err, "failed to construct provider")
		}
		err = providerValidator(provider)
		if err != nil {
			return nil, err
		}

		return wrapError(remote.NewClient(remote.ClientConfig{
			Debug:    cfg.Output.Debug,
			Host:     cfg.Cloud.APIHost,
			Insecure: cfg.Cloud.Insecure,
			Log:      logger,
			Token:    cfg.Secrets.APIToken,
			Provider: provider,
		}))
	}

	if !cfg.Cloud.Disabled {
		logger.Warnf("Unable to find RWX_ACCESS_TOKEN in the environment. rwx test will default to OSS mode.")
		logger.Warnf("You can silence this warning by setting the following in the config file:")
		logger.Warnf("")
		logger.Warnf("cloud:")
		logger.Warnf("  disabled: true")
		logger.Warnf("")
	}

	if cfg.Secrets.APIToken != "" {
		logger.Warnf("rwx test detected an RWX_ACCESS_TOKEN in your environment, however Cloud mode was disabled.")
		logger.Warnf("To start using RWX Cloud, please remove the 'cloud.disabled' setting in the config file.")
	}

	flakesFilePath, err := findInParentDir(filepath.Join(captainDirectory, suiteID, flakesFileName))
	if err != nil {
		flakesFilePath = filepath.Join(captainDirectory, suiteID, flakesFileName)
		logger.Warnf(
			"Unable to find existing flakes.yaml file for suite %q. rwx test will create a new one at %q",
			suiteID, flakesFilePath,
		)
	}

	quarantinesFilePath, err := findInParentDir(filepath.Join(captainDirectory, suiteID, quarantinesFileName))
	if err != nil {
		quarantinesFilePath = filepath.Join(captainDirectory, suiteID, quarantinesFileName)
		logger.Warnf(
			"Unable to find existing quarantines.yaml for suite %q file. rwx test will create a new one at %q",
			suiteID, quarantinesFilePath,
		)
	}

	timingsFilePath, err := findInParentDir(filepath.Join(captainDirectory, suiteID, timingsFileName))
	if err != nil {
		timingsFilePath = filepath.Join(captainDirectory, suiteID, timingsFileName)
		logger.Warnf(
			"Unable to find existing timings.yaml file. rwx test will create a new one at %q",
			timingsFilePath,
		)
	}

	return wrapError(local.NewClient(captainfs.Local{}, flakesFilePath, quarantinesFilePath, timingsFilePath))
}
