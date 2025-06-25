# Contributing to `rwx`

The RWX CLI is an open-source project and we welcome any contributions from other
developers interested in test automation.

## Filing Issues

When opening a new GitHub issue, make sure to include any relevant information,
such as:

* What version of the RWX CLI are you using (see `rwx --version`)?
* What system / CI environment are you running the CLI on?
* What did you do?
* What did you expect to see?
* What did you see instead?

## Contributing Code

We use GitHub pull requests for code contributions and reviews.

Our CI system will run tests & our linting setup against new pull requests, but
to shorten your feedback cycle, we would appreciate if
`go run ./tools/mage lint` and `go run ./tools/mage test` pass before
opening a PR.

### Development setup

You should not need any dependencies outside of Go to work on the RWX CLI.

We use [Mage](https://magefile.org) as the build tool for our project. To show
a list of available targets, run `go run ./tools/mage -l`:

```
Targets:
  all        cleans output, builds, tests, and lints.
  build*     builds the RWX CLI
  clean      removes any generated artifacts from the repository.
  lint       runs the linter & performs static-analysis checks.
  lintFix    Applies lint checks and fixes any issues.
  test       executes the test-suite for the RWX CLI.

* default target
```

Mage can also be installed as command-line applications. This is not strictly
necessary, but is a bit nicer to use.

### Debugging

Besides the `--debug` flag, some useful options during development are:

* `RWX_HOST` to route API traffic to a different host.
