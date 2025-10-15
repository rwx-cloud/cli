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
`golangci-lint run ./...` and `go test ./...` pass before
opening a PR.

### Development setup

We use [mise](https://mise.jdx.dev) to manage local dependencies.

```
mise install
```

### Debugging

Besides the `--debug` flag, some useful options during development are:

* `RWX_HOST` to route API traffic to a different host.
