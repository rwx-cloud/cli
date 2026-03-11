package cli

type LintDiagnostic struct {
	Severity string
}

type LintCheckResult struct {
	Diagnostics []LintDiagnostic
	FileCount   int
}

type LintConfig struct {
	// Check performs the actual lint check. Injected by the command layer
	// to avoid a circular dependency with the lsp package.
	Check func() (*LintCheckResult, error)
	Fix   bool
}

type LintResult struct {
	HasError     bool
	ErrorCount   int
	WarningCount int
	FileCount    int
}

func (s Service) Lint(cfg LintConfig) (*LintResult, error) {
	checkResult, err := cfg.Check()
	if err != nil {
		return nil, err
	}

	errorCount := 0
	warningCount := 0
	for _, d := range checkResult.Diagnostics {
		switch d.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
	}

	return &LintResult{
		HasError:     errorCount > 0,
		ErrorCount:   errorCount,
		WarningCount: warningCount,
		FileCount:    checkResult.FileCount,
	}, nil
}
