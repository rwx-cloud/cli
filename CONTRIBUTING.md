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

### Building with the LSP server

The CLI embeds an LSP (Language Server Protocol) server for `.rwx/` and `.mint/` config files. In CI, the language server is built from the [language-server](https://github.com/rwx-cloud/language-server) repo and embedded automatically. For local development, use the Makefile:

```
make build
```

This clones the language-server repo, compiles it, copies the artifacts into `internal/lsp/bundle/`, and runs `go build`. Subsequent runs skip steps that are already up to date.

To clean up the cloned repo and bundle artifacts:

```
make clean
```

You can also run `go build ./cmd/rwx` directly — the binary will compile without language server artifacts, but `rwx lsp serve` won't function.

### Debugging

Besides the `--debug` flag, some useful options during development are:

* `RWX_HOST` to route API traffic to a different host.
